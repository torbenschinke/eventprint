package heif_test

import (
	"bytes"
	"image"
	"os"
	"testing"

	"github.com/torbenschinke/eventprint/pkg/heif"
)

// sample ist ein 64x48 großes HEIC, erzeugt mit dem Bildwerkzeug des Mac aus
// einem Farbverlauf von Rot nach Blau.
func sample(t *testing.T) []byte {
	t.Helper()

	data, err := os.ReadFile("testdata/sample.heic")
	if err != nil {
		t.Fatal(err)
	}

	return data
}

func TestSniffRecognisesHEIC(t *testing.T) {
	if !heif.Sniff(sample(t)) {
		t.Fatal("HEIC nicht erkannt")
	}

	if heif.Sniff([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatal("JPEG als HEIC erkannt")
	}
}

func TestDecodeHEIC(t *testing.T) {
	if !heif.Register() {
		t.Skip("libheif ist hier nicht installiert")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(sample(t)))
	if err != nil || format != "heif" || cfg.Width != 64 || cfg.Height != 48 {
		t.Fatalf("DecodeConfig = %+v, %q, %v", cfg, format, err)
	}

	img, _, err := image.Decode(bytes.NewReader(sample(t)))
	if err != nil {
		t.Fatal(err)
	}

	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 48 {
		t.Fatalf("Bounds = %v", b)
	}

	// Der Verlauf läuft von Rot oben nach Blau unten.
	r, _, bl, _ := img.At(32, 1).RGBA()
	if r < bl {
		t.Fatalf("oben erwartet rot, bekam r=%d b=%d", r>>8, bl>>8)
	}

	r, _, bl, _ = img.At(32, 46).RGBA()
	if bl < r {
		t.Fatalf("unten erwartet blau, bekam r=%d b=%d", r>>8, bl>>8)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	if !heif.Available() {
		t.Skip("libheif ist hier nicht installiert")
	}

	if _, err := heif.Decode(bytes.NewReader([]byte("kein Bild"))); err == nil {
		t.Fatal("Unsinn wurde dekodiert")
	}
}
