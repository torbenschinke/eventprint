package photo

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // Dekoder für DecodeConfig
	_ "image/png"  // Dekoder für DecodeConfig
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/orient"
)

// ImportCmd beschreibt ein eingehendes Bild.
type ImportCmd struct {
	// Name ist der ursprüngliche Dateiname, sofern bekannt.
	Name string

	Source Source

	// Event ist die laufende Feier; leer im Heimbetrieb.
	Event EventID

	// Unseen legt das Foto in den Eingang.
	Unseen bool

	// Data sind die unveränderten Bytes, so wie Kamera oder Handy sie
	// geliefert haben.
	Data []byte
}

// MaxImportBytes begrenzt ein einzelnes Bild. Größere Dateien sind auf einer
// Speicherkarte kein Foto mehr, sondern ein Problem.
const MaxImportBytes = 60 << 20

// Import übernimmt ein Bild in die Ablage.
//
// Gespeichert wird das Original Byte für Byte, samt EXIF-Block und ohne
// erneute Kompression. Die Lage des Motivs wird nur ausgewertet, nicht in die
// Datei geschrieben: Anzeige und Druck richten es selbst auf, und auf dem
// USB-Stick landet so, was die Kamera tatsächlich geliefert hat.
type Import func(subject permission.Auditable, cmd ImportCmd) (Photo, error)

// NewImport erzeugt den [Import] Anwendungsfall.
func NewImport(mutex *sync.Mutex, repo Repository, originals Originals) Import {
	return func(subject permission.Auditable, cmd ImportCmd) (Photo, error) {
		if err := subject.Audit(PermImport); err != nil {
			return Photo{}, err
		}

		if len(cmd.Data) == 0 {
			return Photo{}, errors.New("die Bilddatei ist leer")
		}

		if len(cmd.Data) > MaxImportBytes {
			return Photo{}, errors.New("die Bilddatei ist zu groß")
		}

		cfg, format, err := image.DecodeConfig(bytes.NewReader(cmd.Data))
		if err != nil {
			// HEIC ist der häufigste Fall: iPhones liefern es, wenn die
			// Freigabe nicht umwandelt. Die Meldung soll sagen, was zu tun ist.
			return Photo{}, errors.New("das Bild kann nicht gelesen werden – bitte als JPEG oder PNG senden")
		}

		width, height := cfg.Width, cfg.Height
		if format == "jpeg" && orient.FromJPEG(cmd.Data).SwapsDimensions() {
			width, height = height, width
		}

		now := time.Now()
		id := NewID(now)

		ext := ".jpg"
		if format == "png" {
			ext = ".png"
		}

		photo := Photo{
			ID:        id,
			Name:      cleanName(cmd.Name, ext),
			File:      string(id) + ext,
			Source:    cmd.Source,
			Event:     cmd.Event,
			Unseen:    cmd.Unseen,
			Width:     width,
			Height:    height,
			CreatedAt: now,
		}

		if err := originals.Put(photo.File, cmd.Data); err != nil {
			return Photo{}, fmt.Errorf("cannot store original: %w", err)
		}

		mutex.Lock()
		defer mutex.Unlock()

		if err := repo.Save(photo); err != nil {
			_ = originals.Remove(photo.File)
			return Photo{}, fmt.Errorf("cannot save photo: %w", err)
		}

		return photo, nil
	}
}

// cleanName macht aus einem mitgelieferten Namen einen brauchbaren
// Dateinamen. Handys liefern gern Pfade oder gar nichts.
func cleanName(name, ext string) string {
	name = strings.TrimSpace(filepath.Base(strings.ReplaceAll(name, "\\", "/")))
	if name == "" || name == "." || name == "/" {
		return ""
	}

	if filepath.Ext(name) == "" {
		name += ext
	}

	return name
}
