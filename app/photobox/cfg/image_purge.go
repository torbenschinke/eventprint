package cfgphotobox

import (
	"context"
	"fmt"

	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/pkg/blob"
	"go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/app/photo"
)

// Namen der Ablagen des Image-Subsystems.
//
// Fest verdrahtet, weil Nago sie nicht nach außen gibt: ImageManagement hält
// Verzeichnis und Blob-Ablage in unexportierten Feldern, und es gibt keinen
// Anwendungsfall zum Löschen. Ohne diesen Zugriff bliebe der Platz belegt,
// obwohl die Fotos verschwunden sind.
//
// Die Namen mit führendem Punkt sind die älteren; Nago prüft sie zuerst und
// legt nur andernfalls die neuen an. Beide Fälle werden hier behandelt, sonst
// räumte die Fotobox auf einem gewachsenen System die falsche Ablage auf.
const (
	imgSetStoreLegacy  = ".nago.img.set"
	imgSetStore        = "nago.img.set"
	imgBlobStoreLegacy = ".nago.img.blob"
	imgBlobStore       = "nago.img.blob"
)

// newPurgeImage öffnet dieselben Ablagen, die auch das Image-Subsystem nutzt.
func newPurgeImage(cfg *application.Configurator) (photo.PurgeImage, error) {
	stores, err := cfg.Stores()
	if err != nil {
		return nil, fmt.Errorf("cannot access stores: %w", err)
	}

	openStore := func(legacy, name string, kind blob.StoreType) (blob.Store, error) {
		opt, err := stores.Get(legacy)
		if err != nil {
			return nil, fmt.Errorf("cannot get store %q: %w", legacy, err)
		}

		if opt.IsSome() {
			return opt.Unwrap(), nil
		}

		return stores.Open(name, blob.OpenStoreOptions{Type: kind})
	}

	setStore, err := openStore(imgSetStoreLegacy, imgSetStore, blob.EntityStore)
	if err != nil {
		return nil, err
	}

	blobStore, err := openStore(imgBlobStoreLegacy, imgBlobStore, blob.FileStore)
	if err != nil {
		return nil, err
	}

	srcSets := json.NewSloppyJSONRepository[image.SrcSet](setStore)

	return func(ctx context.Context, id image.ID) (int64, error) {
		optSet, err := srcSets.FindByID(id)
		if err != nil {
			return 0, fmt.Errorf("cannot read src set %s: %w", id, err)
		}

		if optSet.IsNone() {
			// Kein Verzeichniseintrag: Dann gibt es auch nichts zu löschen.
			// Das ist kein Fehler, sondern der zweite Durchlauf.
			return 0, nil
		}

		var freed int64

		// Ein SrcSet enthält mehrere Größen desselben Bildes und zusätzlich das
		// Original. Jede davon ist ein eigener Blob.
		for _, img := range optSet.Unwrap().Images {
			key := string(img.Data)
			if key == "" {
				continue
			}

			if info, err := blob.Stat(ctx, blobStore, key); err == nil && info.IsSome() {
				if size := info.Unwrap().Size; size > 0 {
					freed += size
				}
			}

			if err := blobStore.Delete(ctx, key); err != nil {
				return freed, fmt.Errorf("cannot delete blob %s: %w", key, err)
			}
		}

		// Erst zuletzt den Verzeichniseintrag: Bricht es vorher ab, findet ein
		// zweiter Versuch die restlichen Blobs noch.
		if err := srcSets.DeleteByID(id); err != nil {
			return freed, fmt.Errorf("cannot delete src set %s: %w", id, err)
		}

		return freed, nil
	}, nil
}
