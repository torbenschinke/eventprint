package photo

import (
	"bytes"
	stdimage "image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/worldiety/speclink/spec"

	"github.com/torbenschinke/eventprint/orient"
	"github.com/torbenschinke/eventprint/requirements/fun/foto"
)

// TestImportArchivesUntouchedOriginal ist die eigentliche Zusage des Archivs.
//
// Der Import richtet ein Bild anhand seiner EXIF-Angabe auf und kodiert es
// dabei neu. Was im Blob-Store landet, ist deshalb nicht mehr die Datei der
// Kamera. Für die Weitergabe nach der Feier ist aber genau die gefragt –
// mitsamt EXIF-Block, Aufnahmezeit und ursprünglicher Kompression.
func TestImportArchivesUntouchedOriginal(t *testing.T) {
	dir := t.TempDir()

	archive, err := NewDirArchive(dir)
	if err != nil {
		t.Fatalf("NewDirArchive: %v", err)
	}

	// Ein Bild, das aufgerichtet werden muss – nur dann weichen Original und
	// gespeicherte Fassung überhaupt voneinander ab.
	raw := jpegRotated(t)

	if o := orient.FromJPEG(raw); o == orient.Normal {
		t.Fatal("das Testbild trägt keine Drehung, der Test prüft nichts")
	}

	var (
		mutex     sync.Mutex
		stored    []byte
		repo      = json.NewSloppyJSONRepository[Photo, ID](mem.NewBlobStore("photo"))
		importing = NewImport(&mutex, events.NewEventBus(), repo, func(_ permission.Auditable, opts image.Options, file image.File) (image.SrcSet, error) {
			var buf bytes.Buffer
			if _, err := file.Transfer(&buf); err != nil {
				return image.SrcSet{}, err
			}

			stored = buf.Bytes()

			return image.SrcSet{ID: opts.ID}, nil
		}, archive)
	)

	p, err := importing(user.SU(), Options{Source: SourceCamera}, image.MemFile{
		Filename: "DSC02301.JPG", MimeTypeHint: "image/jpeg", Bytes: raw,
	})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, string(p.ID)+"_DSC02301.jpg"))
	if err != nil {
		t.Fatalf("archivierte Datei fehlt: %v", err)
	}

	if !bytes.Equal(got, raw) {
		t.Fatal("die archivierte Datei weicht vom Original ab")
	}

	if bytes.Equal(stored, raw) {
		t.Fatal("Voraussetzung entfallen: der Import hat das Bild gar nicht verändert")
	}

	spec.Verified(t, foto.RFotoImport)
}

// TestImportSurvivesBrokenArchive hält fest, dass ein nicht beschreibbares
// Archiv den Abend nicht beendet.
func TestImportSurvivesBrokenArchive(t *testing.T) {
	var mutex sync.Mutex

	repo := json.NewSloppyJSONRepository[Photo, ID](mem.NewBlobStore("photo"))

	broken := Archive(func(ID, string, []byte) error { return os.ErrPermission })

	importing := NewImport(&mutex, events.NewEventBus(), repo, func(_ permission.Auditable, opts image.Options, _ image.File) (image.SrcSet, error) {
		return image.SrcSet{ID: opts.ID}, nil
	}, broken)

	if _, err := importing(user.SU(), Options{}, image.MemFile{
		Filename: "a.jpg", MimeTypeHint: "image/jpeg", Bytes: jpegRotated(t),
	}); err != nil {
		t.Fatalf("Import scheiterte am Archiv: %v", err)
	}
}

// jpegRotated liefert ein JPEG mit gesetztem EXIF-Ausrichtungsfeld.
func jpegRotated(t *testing.T) []byte {
	t.Helper()

	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 8, 4))
	for x := range 8 {
		for y := range 4 {
			img.Set(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 60), B: 0, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	raw := buf.Bytes()

	// Minimaler, aber echter EXIF-Block mit Tag 0x0112 (Orientation) = 6.
	tiff := []byte{
		'I', 'I', 42, 0,
		8, 0, 0, 0,
		1, 0,
		0x12, 0x01,
		3, 0,
		1, 0, 0, 0,
		byte(orient.Rotate90), 0, 0, 0,
		0, 0, 0, 0,
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)

	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	segment = append(segment, payload...)

	out := make([]byte, 0, len(raw)+len(segment))
	out = append(out, raw[:2]...)
	out = append(out, segment...)
	out = append(out, raw[2:]...)

	return out
}
