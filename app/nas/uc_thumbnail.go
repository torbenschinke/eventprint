package nas

import (
	"context"
	"errors"
	"io/fs"
	"path"

	"go.wdy.de/nago/application/permission"
)

// Thumbnail liefert ein Vorschaubild für die Galerie.
//
// Synology Photos und die File Station legen zu jedem Bild fertige
// Vorschauen in "@eaDir/<Datei>/" an. Die zu nehmen spart je Kachel das
// Übertragen und Dekodieren eines Originals mit 5 bis 20 MB, und HEIC-Fotos
// haben dort bereits ein JPEG. Fehlt die Vorschau, etwa auf einem NAS ohne
// Indizierung, kommt das Original.
type Thumbnail func(subject permission.Auditable, ctx context.Context, path string) ([]byte, error)

// Die Vorschauen von Synology. XL misst 1280 Pixel an der langen Kante und
// reicht für eine Kachel auf 1080p; M hat nur 320 und wirkt in einer Reihe von
// 255 Pixel Höhe schon unscharf, ist aber besser als gar nichts.
const (
	thumbXL = "SYNOPHOTO_THUMB_XL.jpg"
	thumbM  = "SYNOPHOTO_THUMB_M.jpg"
)

// NewThumbnail bindet die Vorschau an einen Client und die Einstellungen.
func NewThumbnail(client Client, config func() Config) Thumbnail {
	return func(subject permission.Auditable, ctx context.Context, p string) ([]byte, error) {
		if err := subject.Audit(PermThumbnail); err != nil {
			return nil, err
		}

		cfg := config().Normalized()
		if !cfg.Configured() {
			return nil, ErrNotConfigured
		}

		p, err := cleanPath(p)
		if err != nil {
			return nil, err
		}

		if !isImage(p) {
			return nil, ErrNoImage
		}

		var data []byte
		err = client.Do(ctx, cfg, func(fsys fs.FS) error {
			dir, name := path.Split(p)
			for _, t := range [...]string{thumbXL, thumbM} {
				data, err = readLimited(fsys, path.Join(dir, "@eaDir", name, t), maxReadSize)
				if err == nil {
					return nil
				}

				if !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}

			data, err = readLimited(fsys, p, maxReadSize)
			return err
		})

		return data, err
	}
}
