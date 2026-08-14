package printing_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/torbenschinke/eventprint/printing"
)

// TestToneCurveIsMonotonic sichert die am Gerät gemessenen Kennlinien ab.
//
// Eine nicht monotone Kurve würde Tonwerte vertauschen und im Ausdruck als
// Streifen oder Umkehrungen auffallen.
func TestToneCurveIsMonotonic(t *testing.T) {
	curves := map[string]printing.ToneCurve{
		"Gelb":    printing.CurveYellow,
		"Magenta": printing.CurveMagenta,
		"Cyan":    printing.CurveCyan,
	}

	for name, c := range curves {
		t.Run(name, func(t *testing.T) {
			table := c.Table()

			for v := 1; v < 256; v++ {
				if table[v] < table[v-1] {
					t.Fatalf("Kurve fällt bei %d: %d nach %d", v, table[v-1], table[v])
				}
			}

			// Die Endpunkte müssen erhalten bleiben, sonst verschöbe sich
			// Weiß oder Schwarz.
			if table[0] != 0 {
				t.Errorf("Schwarz = %d, erwartet 0", table[0])
			}

			if table[255] != 255 {
				t.Errorf("Weiß = %d, erwartet 255", table[255])
			}
		})
	}
}

// TestToneCurveLiftsShadows hält die Kernaussage der Messung fest: Der
// Herstellertreiber hebt die Schatten an. Genau das begrenzt die Farbmenge in
// dunklen Partien und verhindert das Ausbluten in Transportrichtung.
func TestToneCurveLiftsShadows(t *testing.T) {
	table := printing.CurveYellow.Table()

	for _, v := range []int{16, 32, 64, 96} {
		if int(table[v]) <= v {
			t.Errorf("Schatten bei %d wird nicht angehoben: %d", v, table[v])
		}
	}

	// Und senkt die Lichter leicht ab.
	for _, v := range []int{208, 224, 240} {
		if int(table[v]) >= v {
			t.Errorf("Licht bei %d wird nicht abgesenkt: %d", v, table[v])
		}
	}
}

// TestRenderAppliesCalibration prüft, dass die Kennlinie im Ausdruck
// tatsächlich ankommt: Ein mittelgrauer Vollton muss verschoben herauskommen.
func TestRenderAppliesCalibration(t *testing.T) {
	const gray = 64

	src := image.NewRGBA(image.Rect(0, 0, 600, 900))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+0], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = gray, gray, gray, 255
	}

	var in bytes.Buffer
	if err := jpeg.Encode(&in, src, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	out, err := printing.Render(&in, printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	b := img.Bounds()
	r, _, _, _ := img.At(b.Dx()/2, b.Dy()/2).RGBA()
	got := int(r >> 8)

	want := int(printing.CurveCyan.Table()[gray])
	if got < want-4 || got > want+4 {
		t.Errorf("Grauwert im Ausdruck = %d, erwartet etwa %d (Kennlinie)", got, want)
	}
}

// TestCompensateDriverRotation sichert die Kompensation eines Gutenprint-
// Fehlers ab.
//
// Für den QW410/CZ-01 ist DYESUB_FEATURE_PLANE_LEFTTORIGHT gesetzt, was jede
// Bildzeile spiegelt, und die Zeilen werden zusätzlich in umgekehrter
// Reihenfolge in das BMP geschrieben. Zusammen ist das eine Drehung um 180
// Grad. Ohne Kompensation kommt jeder Ausdruck auf dem Kopf.
//
// Wird Gutenprint eines Tages berichtigt, schlägt dieser Test nicht fehl –
// dann muss die Kompensation aber entfallen. Der Prüfstein dafür ist der
// Vergleich mit dem Herstellertreiber, siehe README.
func TestCompensateDriverRotation(t *testing.T) {
	// Ein Bild mit eindeutig unterscheidbaren Ecken.
	src := image.NewRGBA(image.Rect(0, 0, 400, 600))
	for y := range 600 {
		for x := range 400 {
			c := color.RGBA{R: 128, G: 128, B: 128, A: 255}
			if x < 100 && y < 100 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255} // oben links weiß
			}

			src.Set(x, y, c)
		}
	}

	var in bytes.Buffer
	if err := jpeg.Encode(&in, src, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	out, err := printing.Render(&in, printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	b := img.Bounds()
	bright := func(x, y int) int {
		r, _, _, _ := img.At(x, y).RGBA()
		return int(r >> 8)
	}

	tl := bright(b.Min.X+40, b.Min.Y+40)
	br := bright(b.Max.X-40, b.Max.Y-40)

	// Die weiße Ecke muss nach der Kompensation unten rechts liegen.
	if br <= tl {
		t.Errorf("weiße Ecke liegt nicht unten rechts: oben links %d, unten rechts %d", tl, br)
	}
}
