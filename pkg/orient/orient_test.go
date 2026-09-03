package orient_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/torbenschinke/eventprint/pkg/orient"
)

// jpegWithOrientation baut ein JPEG mit einem minimalen, aber echten
// EXIF-Block, der genau das Ausrichtungsfeld enthält.
//
// Ein selbst gebautes Testbild ist hier einer Beispieldatei vorzuziehen: Es
// deckt alle acht Fälle ab, während eine reale Kamera immer nur einen
// liefert.
func jpegWithOrientation(t *testing.T, img image.Image, o orient.Orientation) []byte {
	t.Helper()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	raw := buf.Bytes()

	// TIFF-Kopf (little endian), ein Verzeichnis mit einem Eintrag:
	// Tag 0x0112 (Orientation), Typ 3 (SHORT), Anzahl 1, Wert direkt im
	// Eintrag.
	tiff := []byte{
		'I', 'I', 42, 0,
		8, 0, 0, 0, // Verzeichnis beginnt bei Byte 8
		1, 0, // ein Eintrag
		0x12, 0x01, // Tag 0x0112
		3, 0, // Typ SHORT
		1, 0, 0, 0, // Anzahl 1
		byte(o), 0, 0, 0, // Wert
		0, 0, 0, 0, // kein weiteres Verzeichnis
	}

	payload := append([]byte("Exif\x00\x00"), tiff...)

	segment := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte((len(payload) + 2) & 0xFF)}
	segment = append(segment, payload...)

	out := make([]byte, 0, len(raw)+len(segment))
	out = append(out, raw[:2]...) // Startmarker
	out = append(out, segment...)
	out = append(out, raw[2:]...)

	return out
}

// markedImage zeichnet ein Bild, dessen Ecken eindeutig unterscheidbar sind.
// Damit lässt sich prüfen, ob wirklich gedreht und nicht nur beschnitten wurde.
func markedImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}

	// oben links rot markieren
	for y := range h / 4 {
		for x := range w / 4 {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	return img
}

func TestFromJPEGReadsOrientation(t *testing.T) {
	for _, o := range []orient.Orientation{
		orient.Normal,
		orient.FlipHorizontal,
		orient.Rotate180,
		orient.FlipVertical,
		orient.Transpose,
		orient.Rotate90,
		orient.Transverse,
		orient.Rotate270,
	} {
		buf := jpegWithOrientation(t, markedImage(64, 32), o)

		if got := orient.FromJPEG(buf); got != o {
			t.Errorf("FromJPEG = %d, erwartet %d", got, o)
		}
	}
}

// TestFromJPEGWithoutExif deckt den Regelfall der Fotobox mit ab: Ein Bild
// ohne EXIF darf nicht verworfen und nicht gedreht werden.
func TestFromJPEGWithoutExif(t *testing.T) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, markedImage(16, 16), nil); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	if got := orient.FromJPEG(buf.Bytes()); got != orient.Normal {
		t.Errorf("FromJPEG = %d, erwartet %d", got, orient.Normal)
	}
}

// TestFromJPEGRejectsGarbage stellt sicher, dass beschädigte Zusatzdaten
// niemals einen Druck verhindern.
func TestFromJPEGRejectsGarbage(t *testing.T) {
	inputs := [][]byte{
		nil,
		{},
		{0xFF, 0xD8},
		{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x08, 'E', 'x', 'i', 'f'},
		[]byte("überhaupt kein Bild"),
	}

	for _, in := range inputs {
		if got := orient.FromJPEG(in); got != orient.Normal {
			t.Errorf("FromJPEG(% X) = %d, erwartet %d", in, got, orient.Normal)
		}
	}
}

// TestDecodeUprightsPortraitPhoto ist der Fall, der an einer Fotobox
// tatsächlich auftritt: Ein hochkant gehaltenes Smartphone speichert quer und
// vermerkt Rotate90.
func TestDecodeUprightsPortraitPhoto(t *testing.T) {
	// Sensorlage: quer, 64 breit, 32 hoch
	buf := jpegWithOrientation(t, markedImage(64, 32), orient.Rotate90)

	img, o, err := orient.Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if o != orient.Rotate90 {
		t.Errorf("Ausrichtung = %d, erwartet %d", o, orient.Rotate90)
	}

	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 64 {
		t.Fatalf("Abmessungen = %dx%d, erwartet 32x64 – das Bild wurde nicht aufgerichtet", b.Dx(), b.Dy())
	}

	// Die rote Ecke lag oben links und muss nach der Drehung um 90 Grad im
	// Uhrzeigersinn oben rechts liegen.
	if !isRed(img.At(b.Max.X-2, b.Min.Y+2)) {
		t.Error("die Markierung liegt nach dem Aufrichten nicht oben rechts")
	}

	if isRed(img.At(b.Min.X+2, b.Min.Y+2)) {
		t.Error("die Markierung liegt noch oben links, es wurde nicht gedreht")
	}
}

// TestApplyIsIdentityForNormal sichert den schnellen Weg ab: Ohne Drehung
// darf das Bild nicht kopiert werden.
func TestApplyIsIdentityForNormal(t *testing.T) {
	src := markedImage(8, 8)

	if got := orient.Apply(src, orient.Normal); got != image.Image(src) {
		t.Error("Apply hat das Bild ohne Not kopiert")
	}
}

// TestApplyRoundTrip prüft, dass jede Ausrichtung durch ihre Umkehrung wieder
// aufgehoben wird. Das deckt Vorzeichenfehler in der Abbildung auf, die bei
// quadratischen Bildern sonst unbemerkt blieben.
func TestApplyRoundTrip(t *testing.T) {
	inverse := map[orient.Orientation]orient.Orientation{
		orient.FlipHorizontal: orient.FlipHorizontal,
		orient.Rotate180:      orient.Rotate180,
		orient.FlipVertical:   orient.FlipVertical,
		orient.Transpose:      orient.Transpose,
		orient.Rotate90:       orient.Rotate270,
		orient.Transverse:     orient.Transverse,
		orient.Rotate270:      orient.Rotate90,
	}

	src := markedImage(12, 20)

	for o, back := range inverse {
		got := orient.Apply(orient.Apply(src, o), back)

		b := got.Bounds()
		if b.Dx() != 12 || b.Dy() != 20 {
			t.Errorf("Ausrichtung %d: Abmessungen nach Hin- und Rückweg = %dx%d, erwartet 12x20", o, b.Dx(), b.Dy())
			continue
		}

		for y := range 20 {
			for x := range 12 {
				want := src.RGBAAt(x, y)
				r, g, bl, _ := got.At(x, y).RGBA()

				if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(bl>>8) != want.B {
					t.Errorf("Ausrichtung %d: Punkt %d,%d weicht ab", o, x, y)

					y = 20

					break
				}
			}
		}
	}
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()

	return r > 0x8000 && g < 0x4000 && b < 0x4000
}
