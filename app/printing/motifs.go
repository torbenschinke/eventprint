package printing

import (
	"fmt"
	"image"
	"os"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/pkg/orient"
)

// loadMotifs lädt die Originale eines Blattes.
type loadMotifs func(ids []photo.ID, layout Layout) (Sheet, error)

// motifLoader liest die Originale über den Foto-Kontext.
//
// Der Druck läuft ohne jemanden vor dem Bildschirm – der Worker arbeitet
// Aufträge ab, die längst angenommen und geprüft sind. Er liest deshalb als
// System und nicht mit den Rechten dessen, der vielleicht gerade davorsteht.
func motifLoader(locate photo.Locate) loadMotifs {
	return func(ids []photo.ID, layout Layout) (Sheet, error) {
		locs, err := locate(permission.SU(), ids...)
		if err != nil {
			return Sheet{}, err
		}

		if len(locs) == 0 {
			return Sheet{}, errPhotoGone
		}

		sheet := Sheet{Layout: layout}
		for _, loc := range locs {
			raw, err := os.ReadFile(loc.Path)
			if err != nil {
				return Sheet{}, fmt.Errorf("cannot read original: %w", err)
			}

			// Aufgerichtet wird erst hier. Die Datei bleibt das Original,
			// wie Kamera oder Handy es geliefert haben.
			img, _, err := orient.Decode(raw)
			if err != nil {
				return Sheet{}, fmt.Errorf("cannot decode original: %w", err)
			}

			sheet.Images = append(sheet.Images, img)
			sheet.Dates = append(sheet.Dates, loc.Photo.CreatedAt)
		}

		return sheet, nil
	}
}

// placeholderSheet ist ein Blatt ohne Motiv, etwa für die Vorschau eines
// Formats, bevor ein Foto gewählt ist.
func placeholderSheet(layout Layout) Sheet {
	return Sheet{Layout: layout, Images: []image.Image{placeholderImage()}, Dates: []time.Time{time.Now()}}
}

// placeholderImage ist ein graues Motiv in Hochformat. Ein image.Uniform
// taugt dafür nicht: Seine Grenzen sind praktisch unendlich, und das
// Skalieren darauf würde einen Speicherbedarf jenseits jeder Karte anmelden.
func placeholderImage() image.Image {
	img := image.NewGray(image.Rect(0, 0, 400, 600))
	for i := range img.Pix {
		img.Pix[i] = 0xB0
	}

	return img
}

// visibleTo prüft, ob der Aufrufer die Fotos sehen darf.
//
// Gerendert und gedruckt wird als System, weil der Worker ohne jemanden vor
// dem Bildschirm arbeitet. Ob ein Foto überhaupt gedruckt werden darf,
// entscheidet aber der, der den Druck auslöst: Ein Gast im Kiosk darf private
// Fotos nicht drucken, auch wenn er ihre Kennung aus der Auftragsliste kennt.
func visibleTo(locate photo.Locate) func(permission.Auditable, []photo.ID) error {
	return func(subject permission.Auditable, ids []photo.ID) error {
		locs, err := locate(subject, ids...)
		if err != nil {
			return err
		}

		seen := map[photo.ID]bool{}
		for _, l := range locs {
			seen[l.Photo.ID] = true
		}

		for _, id := range ids {
			if !seen[id] {
				return errPhotoGone
			}
		}

		return nil
	}
}
