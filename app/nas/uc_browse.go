package nas

import (
	"context"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"go.wdy.de/nago/application/permission"
)

// Browse listet einen Ordner der eingerichteten Freigabe: Unterordner und
// druckbare Bilder, jeweils nach Namen. Kameras und Telefone nummerieren
// fortlaufend, die Reihenfolge nach Namen ist deshalb auch die der Aufnahme.
type Browse func(subject permission.Auditable, ctx context.Context, dir string) (Listing, error)

// Listing ist der Inhalt eines Ordners.
type Listing struct {
	// Dir ist der Ordner relativ zur Freigabe, "." für die Wurzel.
	Dir     string
	Folders []Folder
	Images  []Image

	// Truncated meldet, dass der Ordner mehr Bilder hat, als gezeigt werden.
	Truncated bool
}

// Folder ist ein Unterordner.
type Folder struct {
	Path string
	Name string
}

// Image ist ein Bild auf dem NAS.
type Image struct {
	// Path ist relativ zur Freigabe, so wie [Read] und [Thumbnail] ihn
	// erwarten.
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// maxImages begrenzt einen Ordner. Wer 30 000 Fotos in einen Ordner legt,
// soll trotzdem eine bedienbare Galerie bekommen und nicht eine, die den Pi
// minutenlang beschäftigt.
const maxImages = 5000

// NewBrowse bindet die Ordneransicht an einen Client und die Einstellungen.
func NewBrowse(client Client, config func() Config) Browse {
	return func(subject permission.Auditable, ctx context.Context, dir string) (Listing, error) {
		if err := subject.Audit(PermBrowse); err != nil {
			return Listing{}, err
		}

		cfg := config().Normalized()
		if !cfg.Configured() {
			return Listing{}, ErrNotConfigured
		}

		dir, err := cleanPath(dir)
		if err != nil {
			return Listing{}, err
		}

		l := Listing{Dir: dir}
		err = client.Do(ctx, cfg, func(fsys fs.FS) error {
			entries, err := fs.ReadDir(fsys, dir)
			if err != nil {
				return err
			}

			l.Folders, l.Images = nil, nil
			for _, e := range entries {
				name := e.Name()
				if hidden(name) {
					continue
				}

				p := path.Join(dir, name)
				if e.IsDir() {
					l.Folders = append(l.Folders, Folder{Path: p, Name: name})
					continue
				}

				if !e.Type().IsRegular() || !isImage(name) {
					continue
				}

				img := Image{Path: p, Name: name}
				if info, err := e.Info(); err == nil {
					img.Size, img.ModTime = info.Size(), info.ModTime()
				}

				l.Images = append(l.Images, img)
			}

			return nil
		})
		if err != nil {
			return Listing{}, err
		}

		slices.SortFunc(l.Folders, func(a, b Folder) int { return byName(a.Name, b.Name) })
		slices.SortFunc(l.Images, func(a, b Image) int { return byName(a.Name, b.Name) })

		if len(l.Images) > maxImages {
			l.Images, l.Truncated = l.Images[:maxImages], true
		}

		return l, nil
	}
}

func byName(a, b string) int {
	if c := strings.Compare(strings.ToLower(a), strings.ToLower(b)); c != 0 {
		return c
	}

	return strings.Compare(a, b)
}
