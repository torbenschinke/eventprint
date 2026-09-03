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

// imageStores bündelt den Zugriff auf die Bildablage von Nago.
type imageStores struct {
	// Purge entfernt die Bilddaten eines Fotos.
	Purge photo.PurgeImage

	// Bytes ist der Platzbedarf aller Bilddaten.
	Bytes func() (int64, error)
}

// newImageStores öffnet dieselben Ablagen, die auch das Image-Subsystem nutzt.
func newImageStores(cfg *application.Configurator) (imageStores, error) {
	stores, err := cfg.Stores()
	if err != nil {
		return imageStores{}, fmt.Errorf("cannot access stores: %w", err)
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
		return imageStores{}, err
	}

	blobStore, err := openStore(imgBlobStoreLegacy, imgBlobStore, blob.FileStore)
	if err != nil {
		return imageStores{}, err
	}

	srcSets := json.NewSloppyJSONRepository[image.SrcSet](setStore)

	purge := func(ctx context.Context, id image.ID) (int64, error) {
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
	}

	// bytes fragt die Ablage und nicht das Dateisystem.
	//
	// Ein du über das Datenverzeichnis wäre grob irreführend: Dort liegen auch
	// die Zwischenspeicher des Go-Übersetzers. Auf der Fotobox waren das 2,3 GB
	// gegenüber 2,8 MB echter Bilddaten. Wer das als "Bildablage" ausweist,
	// verspricht beim Aufräumen Platz, der nicht frei wird.
	bytes := func() (int64, error) {
		ctx := context.Background()

		var total int64

		for key, err := range blobStore.List(ctx, blob.ListOptions{}) {
			if err != nil {
				return total, fmt.Errorf("cannot list image blobs: %w", err)
			}

			info, err := blob.Stat(ctx, blobStore, key)
			if err != nil || info.IsNone() {
				continue
			}

			if size := info.Unwrap().Size; size > 0 {
				total += size
			}
		}

		return total, nil
	}

	return imageStores{Purge: purge, Bytes: bytes}, nil
}
