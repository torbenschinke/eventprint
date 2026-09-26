package photo_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"

	"github.com/torbenschinke/eventprint/app/photo"
)

var su = permission.SU()

// library ist eine Mediathek wie im Betrieb: JSON-Repository und ein echtes
// Verzeichnis für die Originale. Nur der Blob-Store liegt im Speicher – die
// Serialisierung zählt, denn ein Feld, das nicht durch JSON kommt, wäre nach
// einem Neustart verloren.
type library struct {
	uc   photo.UseCases
	repo photo.Repository
	dir  string
}

func newLibrary(t *testing.T) library {
	t.Helper()

	dir := t.TempDir()
	originals, err := photo.NewDirOriginals(dir)
	if err != nil {
		t.Fatal(err)
	}

	repo := json.NewSloppyJSONRepository[photo.Photo, photo.ID](mem.NewBlobStore("photo"))

	return library{uc: photo.NewUseCases(repo, originals), repo: repo, dir: dir}
}

// importOne übernimmt ein Bild und wartet danach kurz.
//
// Die IDs sind nur auf die Millisekunde sortierbar. Zwei Importe in derselben
// Millisekunde stünden in zufälliger Reihenfolge – für Tests, die "neueste
// zuerst" prüfen, wäre das ein Würfelwurf.
func (l library) importOne(t *testing.T, cmd photo.ImportCmd) photo.Photo {
	t.Helper()

	if cmd.Data == nil {
		cmd.Data = jpegBytes(t, 40, 30)
	}

	p, err := l.uc.Import(su, cmd)
	if err != nil {
		t.Fatalf("Import(%q): %v", cmd.Name, err)
	}

	time.Sleep(2 * time.Millisecond)

	return p
}

// stored liest die abgelegte Datei direkt von der Platte – an der Mediathek
// vorbei, damit der Test nicht dem Code glaubt, den er prüft.
func (l library) stored(t *testing.T, p photo.Photo) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(l.dir, p.File))
	if err != nil {
		t.Fatalf("Original %s: %v", p.File, err)
	}

	return data
}

func (l library) exists(p photo.Photo) bool {
	_, err := os.Stat(filepath.Join(l.dir, p.File))
	return err == nil
}

func readAll(t *testing.T, r io.ReadCloser) []byte {
	t.Helper()
	defer r.Close()

	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func ids(photos []photo.Photo) []photo.ID {
	out := make([]photo.ID, 0, len(photos))
	for _, p := range photos {
		out = append(out, p.ID)
	}

	return out
}

func gradient(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 90, A: 255})
		}
	}

	return img
}

func jpegBytes(t *testing.T, w, h int) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gradient(w, h), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := png.Encode(&buf, gradient(w, h)); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

// jpegWithOrientation hängt einen minimalen EXIF-Block mit dem
// Orientation-Tag an, so wie ihn ein hochkant gehaltenes Handy schreibt: Die
// Pixel liegen quer, das Tag sagt "um 90° drehen".
func jpegWithOrientation(t *testing.T, w, h int, orientation byte) []byte {
	t.Helper()

	raw := jpegBytes(t, w, h)
	tiff := []byte{
		'I', 'I', 42, 0,
		8, 0, 0, 0,
		1, 0,
		0x12, 0x01,
		3, 0,
		1, 0, 0, 0,
		orientation, 0, 0, 0,
		0, 0, 0, 0,
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	segment = append(segment, payload...)

	out := append([]byte{}, raw[:2]...)
	out = append(out, segment...)
	return append(out, raw[2:]...)
}

// subject ist ein Nutzer mit genau den genannten Berechtigungen, etwa ein
// Gast im Kiosk.
type subject map[permission.ID]bool

func allow(perms ...permission.ID) subject {
	s := subject{}
	for _, p := range perms {
		s[p] = true
	}

	return s
}

func (s subject) Audit(id permission.ID) error {
	if s[id] {
		return nil
	}

	return fmt.Errorf("permission denied: %s", id)
}

func (s subject) HasPermission(id permission.ID) bool { return s[id] }
