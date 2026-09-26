package nas

import (
	"context"
	"io"
	"io/fs"

	"go.wdy.de/nago/application/permission"
)

// Read lädt ein Bild vom NAS, um es auf die Box zu übernehmen.
type Read func(subject permission.Auditable, ctx context.Context, path string) ([]byte, error)

// maxReadSize begrenzt ein Bild wie beim USB-Stick. Ein 50-Megapixel-JPEG hat
// rund 25 MB; was darüber liegt, würde beim Dekodieren den Speicher des Pi
// sprengen.
const maxReadSize = 60 << 20

// NewRead bindet das Laden an einen Client und die Einstellungen.
func NewRead(client Client, config func() Config) Read {
	return func(subject permission.Auditable, ctx context.Context, p string) ([]byte, error) {
		if err := subject.Audit(PermRead); err != nil {
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
			data, err = readLimited(fsys, p, maxReadSize)
			return err
		})

		return data, err
	}
}

// readLimited liest eine Datei, aber nicht mehr als limit Bytes.
func readLimited(fsys fs.FS, name string, limit int64) ([]byte, error) {
	f, err := fsys.Open(name)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	if info, err := f.Stat(); err == nil && info.Size() > limit {
		return nil, ErrTooLarge
	}

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}

	return data, nil
}
