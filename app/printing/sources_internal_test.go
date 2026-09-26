package printing

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

// TestPreviewLoadsReducedAndOnce: Die Vorschau dekodiert ein großes Original
// verkleinert, und ein zweites Rendern mit demselben Foto liest es gar nicht
// mehr – so bleibt der Tipp auf ein anderes Format schnell.
func TestPreviewLoadsReducedAndOnce(t *testing.T) {
	big := image.NewRGBA(image.Rect(0, 0, 3200, 2000))
	for i := range big.Pix {
		big.Pix[i] = 0x80
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, big, nil); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "big.jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	locates := 0
	locate := func(_ permission.Auditable, ids ...photo.ID) ([]photo.Location, error) {
		locates++
		return []photo.Location{{Photo: photo.Photo{ID: "big"}, Path: path}}, nil
	}

	var asked [2]int
	scaled := func(raw []byte, minW, minH int) (image.Image, error) {
		asked = [2]int{minW, minH}
		// Wie libjpeg-turbo: der nächste Achtelschritt, der reicht.
		return image.NewRGBA(image.Rect(0, 0, 3200/2, 2000/2)), nil
	}

	cache := newPreviewSources(previewCacheSize)
	load := previewLoader(locate, scaled, cache)

	sheet, err := load([]photo.ID{"big"}, DefaultLayout())
	if err != nil {
		t.Fatal(err)
	}

	if asked != [2]int{PreviewSourceEdge, 1000} {
		t.Fatalf("scaled decoder asked for %v, want at least %dx1000", asked, PreviewSourceEdge)
	}

	if b := sheet.Images[0].Bounds(); max(b.Dx(), b.Dy()) > PreviewSourceEdge {
		t.Fatalf("preview source is %v, want the long edge at most %d", b, PreviewSourceEdge)
	}

	// Das Original verschwindet von der Karte: Die Vorschau braucht es nicht
	// mehr, sie hat es im Speicher.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	again, err := load([]photo.ID{"big"}, DefaultLayout())
	if err != nil {
		t.Fatalf("second preview must come from the cache: %v", err)
	}

	if locates != 1 || again.Images[0] != sheet.Images[0] {
		t.Fatalf("second preview located %d times, same image %v", locates, again.Images[0] == sheet.Images[0])
	}

	// Gesichter werden je Quelle einmal gesucht, nicht bei jeder Vorschau.
	detects := 0
	detect := cache.detector(func(image.Image) []image.Rectangle {
		detects++
		return []image.Rectangle{image.Rect(1, 1, 2, 2)}
	})

	detect(sheet.Images[0])
	detect(sheet.Images[0])
	if detects != 1 {
		t.Fatalf("faces detected %d times, want once", detects)
	}

	spec.Verified(t, druck.RDruckVorschau)
}

// TestDecodeOriginalWithoutScaledDecoder: Ohne libjpeg-turbo wird voll
// dekodiert und danach verkleinert – langsamer, aber gleich im Ergebnis.
func TestDecodeOriginalWithoutScaledDecoder(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2400, 1200))
	for i := range img.Pix {
		img.Pix[i] = 0xFF
	}

	img.Set(0, 0, color.Black)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}

	out, err := decodeOriginal(buf.Bytes(), 1200, nil)
	if err != nil {
		t.Fatal(err)
	}

	if b := out.Bounds(); b.Dx() != 1200 || b.Dy() != 600 {
		t.Fatalf("reduced to %v, want 1200x600", b)
	}

	full, err := decodeOriginal(buf.Bytes(), 0, nil)
	if err != nil {
		t.Fatal(err)
	}

	if b := full.Bounds(); b.Dx() != 2400 {
		t.Fatalf("print decode must keep the full resolution, got %v", b)
	}

	spec.Verified(t, druck.RDruckVorschau)
}
