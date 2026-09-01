package photo

import (
	"bytes"
	"fmt"
	stdimage "image"
	"image/jpeg"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/torbenschinke/eventprint/orient"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"
	"go.wdy.de/nago/pkg/std"
)

// NewImport erzeugt den [Import] Anwendungsfall.
//
// archive darf nil sein; dann wird nicht gesichert.
func NewImport(mutex *sync.Mutex, bus events.Bus, repo Repository, createSrcSet image.CreateSrcSet, archive Archive) Import {
	return func(subject auth.Subject, opts Options, file image.File) (Photo, error) {
		if err := subject.Audit(PermImport); err != nil {
			return Photo{}, err
		}

		if file == nil {
			return Photo{}, std.NewLocalizedError("Kein Bild", "Es wurde keine Bilddatei übergeben.")
		}

		if mime, ok := file.MimeType(); ok && !strings.HasPrefix(mime, "image/") {
			return Photo{}, std.NewLocalizedError("Falscher Dateityp", "Es können nur Bilddateien gedruckt werden.")
		}

		now := time.Now()
		id := NewID(now)

		var buf bytes.Buffer
		if _, err := file.Transfer(&buf); err != nil {
			return Photo{}, fmt.Errorf("cannot read image: %w", err)
		}

		raw := buf.Bytes()

		// Gesichert wird vor jeder Verarbeitung. Was hier abgelegt wird, ist
		// exakt das, was Kamera oder Smartphone geliefert haben – inklusive
		// EXIF-Block und ohne erneute Kompression.
		if archive != nil {
			if err := archive(id, file.Name(), raw); err != nil {
				// Bewusst kein Abbruch: Das Archiv ist eine Zugabe für nach
				// der Feier, der Druck ist der Zweck des Abends.
				slog.Error("cannot archive original photo", "photo", string(id), "name", file.Name(), "err", err)
			}
		}

		upright, err := uprightBytes(raw, file.Name(), mimeTypeOf(file))
		if err != nil {
			return Photo{}, err
		}

		// Das Image-Subsystem legt das Original unter der SrcSet-ID ab und
		// erzeugt zusätzlich verkleinerte Varianten für die Anzeige.
		srcSet, err := createSrcSet(subject, image.Options{ID: image.ID(id)}, upright)
		if err != nil {
			return Photo{}, fmt.Errorf("cannot create src set: %w", err)
		}

		source := opts.Source
		if source == "" {
			source = SourceUpload
		}

		photo := Photo{
			ID:        id,
			Image:     srcSet.ID,
			Name:      filepath.Base(file.Name()),
			Source:    source,
			CreatedAt: now,
		}

		if len(srcSet.Images) > 0 {
			photo.Width = srcSet.Images[0].Width
			photo.Height = srcSet.Images[0].Height
		}

		mutex.Lock()
		defer mutex.Unlock()

		if err := repo.Save(photo); err != nil {
			return Photo{}, fmt.Errorf("cannot save photo: %w", err)
		}

		bus.Publish(Imported{Photo: photo.ID, Source: photo.Source})

		return photo, nil
	}
}

// uprightFile richtet ein Bild anhand seiner EXIF-Angabe auf, bevor es
// gespeichert wird.
//
// Die Normalisierung gehört bewusst an den Import und nicht an den Druck:
// Nur so zeigen Vorschau, Historie und Ausdruck dieselbe Lage. Das
// Image-Subsystem von Nago wertet EXIF nämlich nicht aus und verwirft den
// Block beim Erzeugen der Vorschaubilder – ein hochkant gehaltenes Smartphone
// erschiene sonst in der Galerie liegend, während der Browser das Original
// aufrichtet.
//
// Bilder ohne Drehung werden unverändert durchgereicht und dabei nicht neu
// komprimiert.
func uprightBytes(raw []byte, name, mime string) (image.File, error) {
	o := orient.FromJPEG(raw)
	if o == orient.Normal {
		return image.MemFile{
			Filename:     name,
			MimeTypeHint: mime,
			Bytes:        raw,
		}, nil
	}

	img, _, err := stdimage.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, std.NewLocalizedError("Unlesbares Bild", "Die Datei konnte nicht als Bild gelesen werden.")
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, orient.Apply(img, o), &jpeg.Options{Quality: uprightQuality}); err != nil {
		return nil, fmt.Errorf("cannot encode upright image: %w", err)
	}

	return image.MemFile{
		Filename:     name,
		MimeTypeHint: "image/jpeg",
		Bytes:        out.Bytes(),
	}, nil
}

// uprightQuality ist hoch angesetzt, weil das aufgerichtete Bild die Vorlage
// für den Druck ist und nur dieses eine Mal neu komprimiert wird.
const uprightQuality = 95

func mimeTypeOf(file image.File) string {
	if mime, ok := file.MimeType(); ok {
		return mime
	}

	return "image/jpeg"
}
