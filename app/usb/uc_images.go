package usb

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.wdy.de/nago/application/permission"
)

// Images listet die Bilder auf einem Stick, neueste zuerst.
type Images func(subject permission.Auditable, ctx context.Context, drive string) ([]Image, error)

// Image ist ein Bild auf dem Stick.
type Image struct {
	// Path ist der absolute Pfad, so wie ihn [Read] erwartet.
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
}

// maxImages begrenzt die Liste. Wer seine ganze Fotosammlung auf dem Stick
// hat, will trotzdem nur ein paar Bilder drucken, und eine Liste mit
// Hunderttausenden Einträgen legt die Oberfläche auf dem Pi lahm.
const maxImages = 5000

// maxDepth begrenzt die Ordnertiefe. DCIM/100CANON/ ist zwei Ebenen tief,
// sortierte Archive selten mehr als vier. Tiefer liegt meist Programmcode
// oder ein Backup, und das Durchsuchen dauert auf einem Stick lange.
const maxDepth = 6

// NewImages bindet die Bildersuche an das eingehängte Dateisystem.
//
// Nur JPEG und PNG: Das sind die Formate, die die Druckstrecke lesen kann.
// HEIC vom iPhone ließe sich hier zwar finden, aber nicht drucken, und ein
// Bild, das beim Antippen scheitert, ist schlimmer als eines, das fehlt.
func NewImages(runner Runner) Images {
	h := host{runner: runner}

	return func(subject permission.Auditable, ctx context.Context, drive string) ([]Image, error) {
		if err := subject.Audit(PermImages); err != nil {
			return nil, err
		}

		d, err := h.find(ctx, drive)
		if err != nil {
			return nil, err
		}

		mp, err := h.mount(ctx, d)
		if err != nil {
			return nil, err
		}

		return findImages(ctx, mp)
	}
}

// findImages durchsucht einen eingehängten Stick.
func findImages(ctx context.Context, root string) ([]Image, error) {
	var images []Image

	err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}

		// Ein unlesbarer Ordner, etwa ein beschädigter Verzeichniseintrag
		// auf einem oft ohne Auswerfen gezogenen Stick, soll nicht die
		// ganze Liste verhindern.
		if err != nil {
			if e != nil && e.IsDir() && path != root {
				return fs.SkipDir
			}

			if path == root {
				return err
			}

			return nil
		}

		if e.IsDir() {
			if path == root {
				return nil
			}

			if skipDir(e.Name()) || depth(root, path) > maxDepth {
				return fs.SkipDir
			}

			return nil
		}

		// Versteckte Dateien überspringen: macOS legt zu jedem Foto auf FAT
		// ein "._IMG_0001.JPG" mit Metadaten an, das wie ein Bild heißt,
		// aber keins ist.
		if strings.HasPrefix(e.Name(), ".") || !e.Type().IsRegular() || !isImageName(e.Name()) {
			return nil
		}

		info, err := e.Info()
		if err != nil {
			return nil
		}

		images = append(images, Image{
			Path:    path,
			Name:    e.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(images, func(i, j int) bool {
		if !images[i].ModTime.Equal(images[j].ModTime) {
			return images[i].ModTime.After(images[j].ModTime)
		}

		return images[i].Path < images[j].Path
	})

	if len(images) > maxImages {
		images = images[:maxImages]
	}

	return images, nil
}

// skipDir erkennt Ordner, in denen nie Fotos des Gastgebers liegen:
// versteckte Ordner und die Verwaltungsordner von Windows und macOS. Im
// Papierkorb liegen zwar Bilder, aber gelöschte, und die wiederzusehen
// überrascht niemanden angenehm.
func skipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}

	switch name {
	case "System Volume Information", "$RECYCLE.BIN", "RECYCLER", ".Trashes", ".Spotlight-V100", ".fseventsd", "lost+found":
		return true
	default:
		return false
	}
}

// depth zählt die Ordnerebenen unterhalb von root.
func depth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return maxDepth + 1
	}

	return len(strings.Split(filepath.ToSlash(rel), "/"))
}

// isImageName erkennt die druckbaren Formate an der Endung, unabhängig von
// der Schreibweise: Kameras schreiben ".JPG", Telefone ".jpg".
func isImageName(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}
