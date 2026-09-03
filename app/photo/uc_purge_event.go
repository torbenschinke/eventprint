package photo

import (
	"context"
	"fmt"
	"sync"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"
)

// PurgeImage entfernt die Bilddaten eines Fotos aus der Ablage.
//
// Eine Naht zum Image-Subsystem von Nago, das selbst keinen Anwendungsfall zum
// Löschen anbietet. Sie hält [PurgeEvent] prüfbar, ohne dass ein Test eine
// echte Blob-Ablage aufbauen müsste, und hält die Kenntnis der Blob-Schlüssel
// dort, wo die Ablage aufgebaut wird.
//
// Der Rückgabewert ist der freigewordene Platz.
type PurgeImage func(ctx context.Context, id image.ID) (int64, error)

// PurgeResult beschreibt, was eine Bereinigung entfernt hat.
type PurgeResult struct {
	// Photos ist die Anzahl der entfernten Fotos.
	Photos int

	// ImageBytes ist der in der Bildablage freigewordene Platz.
	ImageBytes int64

	// Archive beschreibt die zusätzlich entfernten Archivdateien.
	Archive ArchiveUsage
}

// FreedBytes ist der insgesamt freigewordene Platz.
func (r PurgeResult) FreedBytes() int64 { return r.ImageBytes + r.Archive.Bytes }

// PurgeEvent macht die Fotobox für die nächste Veranstaltung frei.
//
// Anders als [Delete], das bewusst nur die Metadaten entfernt und die
// Bilddaten für den Fall eines Versehens liegen lässt, entfernt dieser
// Anwendungsfall alles: Fotos, Bilddaten und das Archiv. Genau darum geht es –
// die Feier ist vorbei, die Bilder sind heruntergeladen, und die Speicherkarte
// soll wieder leer sein.
//
// Weil danach nichts wiederherstellbar ist, gehört vor den Aufruf eine
// Rückfrage, die sich nicht versehentlich wegklicken lässt.
type PurgeEvent func(subject auth.Subject) (PurgeResult, error)

// NewPurgeEvent erzeugt den [PurgeEvent] Anwendungsfall.
func NewPurgeEvent(mutex *sync.Mutex, bus events.Bus, repo Repository, purgeImage PurgeImage, archiveDir string) PurgeEvent {
	return func(subject auth.Subject) (PurgeResult, error) {
		if err := subject.Audit(PermPurgeEvent); err != nil {
			return PurgeResult{}, err
		}

		mutex.Lock()
		defer mutex.Unlock()

		ctx := context.Background()

		var result PurgeResult

		// Erst einsammeln, dann löschen: Über ein Verzeichnis zu laufen und es
		// dabei zu verändern, ist der klassische Weg, Einträge zu überspringen.
		var photos []Photo
		for p, err := range repo.All() {
			if err != nil {
				return result, fmt.Errorf("cannot read photos: %w", err)
			}

			photos = append(photos, p)
		}

		for _, p := range photos {
			// Zuerst die Bilddaten. Scheitert das, bleibt der Eintrag stehen
			// und der Versuch ist wiederholbar. Andersherum entstünde ein
			// verwaister Blob, den niemand mehr findet.
			if purgeImage != nil && p.Image != "" {
				freed, err := purgeImage(ctx, p.Image)
				if err != nil {
					return result, fmt.Errorf("cannot delete image data of %s: %w", p.ID, err)
				}

				result.ImageBytes += freed
			}

			if err := repo.DeleteByID(p.ID); err != nil {
				return result, fmt.Errorf("cannot delete photo %s: %w", p.ID, err)
			}

			bus.Publish(Deleted{Photo: p.ID})

			result.Photos++
		}

		// Das Archiv zuletzt: Es ist die Kopie, die an die Gäste geht. Wer
		// hier abbricht, hat sie im Zweifel noch.
		if archiveDir != "" {
			freed, err := purgeDir(archiveDir)
			if err != nil {
				return result, err
			}

			result.Archive = freed
		}

		return result, nil
	}
}
