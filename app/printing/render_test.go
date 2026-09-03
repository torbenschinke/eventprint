package printing_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/torbenschinke/eventprint/app/printing"
)

// sampleImage ist ein echtes Kamerabild im Hochformat mit 16:9 – also gerade
// nicht dem Seitenverhältnis des Papiers. Genau dieser Fall erzeugt beim
// naiven Drucken die weißen Balken, die es zu vermeiden gilt.
const sampleImage = "../../DSC02301.jpg"

func loadSample(t *testing.T) []byte {
	t.Helper()

	// Kein Skip: Das Bild liegt im Repository. Fehlt es, ist der Pfad falsch,
	// und ein übersprungener Test sähe aus wie ein bestandener. Genau so blieb
	// nach dem Umzug der Pakete unbemerkt, dass hier gar nichts mehr lief.
	buf, err := os.ReadFile(sampleImage)
	if err != nil {
		t.Fatalf("Beispielbild nicht lesbar: %v", err)
	}

	return buf
}

func renderSample(t *testing.T, tpl printing.TemplateID) image.Image {
	t.Helper()

	out, err := printing.Render(bytes.NewReader(loadSample(t)), tpl, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render(%s): %v", tpl, err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	// Zum Ansehen des Ergebnisses: go test ./printing -run TestRender -keep
	if dir := os.Getenv("EVENTPRINT_TEST_OUTPUT"); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, string(tpl)+".jpg"), out, 0o644)
	}

	return img
}

// TestRenderKeepsPaperGeometry stellt sicher, dass jedes Layout exakt das
// Papierformat trifft. Weicht die Geometrie ab, skaliert CUPS nach und es
// entstehen wieder die unerwünschten Ränder.
func TestRenderKeepsPaperGeometry(t *testing.T) {
	// Die native Rastergröße des Druckers, nicht die rechnerischen 1200x1800.
	wantShort := printing.NativeRaster4x6.Short()
	wantLong := printing.NativeRaster4x6.Long()

	for _, tpl := range printing.Templates() {
		t.Run(string(tpl.ID), func(t *testing.T) {
			img := renderSample(t, tpl.ID)

			b := img.Bounds()
			// Das Beispielbild ist hochkant, also muss auch das Papier
			// hochkant belegt werden.
			if b.Dx() != wantShort || b.Dy() != wantLong {
				t.Errorf("Abmessungen = %dx%d, erwartet %dx%d", b.Dx(), b.Dy(), wantShort, wantLong)
			}
		})
	}
}

// TestRenderFullHasNoBorder prüft die Kernanforderung der Fotobox: Das
// formatfüllende Layout darf an keiner Kante Papier durchscheinen lassen.
func TestRenderFullHasNoBorder(t *testing.T) {
	img := renderSample(t, printing.TemplateFull)
	b := img.Bounds()

	corners := map[string]image.Point{
		"oben links":   {X: b.Min.X + 2, Y: b.Min.Y + 2},
		"oben rechts":  {X: b.Max.X - 3, Y: b.Min.Y + 2},
		"unten links":  {X: b.Min.X + 2, Y: b.Max.Y - 3},
		"unten rechts": {X: b.Max.X - 3, Y: b.Max.Y - 3},
	}

	for name, pt := range corners {
		if isNearWhite(img.At(pt.X, pt.Y)) {
			t.Errorf("Ecke %s ist weiß – das Motiv füllt das Papier nicht aus", name)
		}
	}
}

// TestRenderPassepartoutHasWhiteMargin ist die Gegenprobe: Beim Passepartout
// muss ringsum Papier sichtbar bleiben, während die Bildmitte belegt ist.
func TestRenderPassepartoutHasWhiteMargin(t *testing.T) {
	img := renderSample(t, printing.TemplatePassepartout)
	b := img.Bounds()

	if !isNearWhite(img.At(b.Min.X+2, b.Min.Y+2)) {
		t.Error("obere linke Ecke ist nicht weiß – der Rand fehlt")
	}

	if isNearWhite(img.At(b.Dx()/2, b.Dy()/2)) {
		t.Error("Bildmitte ist weiß – das Motiv fehlt")
	}
}

// TestRenderPolaroidHasWideBottomBar prüft den Sofortbild-Look: Der Steg
// muss deutlich breiter sein als der gegenüberliegende Rand.
//
// Achtung auf die Orientierung: Render liefert die Seite so, wie der
// Druckertreiber sie erwartet, und der dreht sie um 180 Grad (siehe
// compensateDriverRotation). Im Ergebnis liegt der breite Steg deshalb
// **oben** — auf dem Papier landet er unten. Genau das prüft dieser Test mit.
func TestRenderPolaroidHasWideBottomBar(t *testing.T) {
	img := renderSample(t, printing.TemplatePolaroid)
	b := img.Bounds()

	x := b.Dx() / 2
	top := whiteRunFromTop(img, x)
	bottom := whiteRunFromBottom(img, x)

	if top <= bottom*2 {
		t.Errorf("Steg (%d px oben in den Druckdaten) ist nicht deutlich breiter als der Rand (%d px)", top, bottom)
	}
}

// TestRenderLandscapePolaroidKeepsPortraitPaper sichert ab, dass ein
// Querformatmotiv nicht das Polaroid selbst dreht. Der breite Steg bleibt so
// an derselben kurzen Papierkante wie in der Vorschau.
func TestRenderLandscapePolaroidKeepsPortraitPaper(t *testing.T) {
	raw := jpegWithOrientation(t, 900, 600, 0)
	out, err := printing.Render(bytes.NewReader(raw), printing.TemplatePolaroid, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != printing.NativeRaster4x6.Short() || b.Dy() != printing.NativeRaster4x6.Long() {
		t.Fatalf("Polaroid-Abmessungen = %dx%d, erwartet Hochformat %dx%d", b.Dx(), b.Dy(), printing.NativeRaster4x6.Short(), printing.NativeRaster4x6.Long())
	}

	x := b.Dx() / 2
	if top, bottom := whiteRunFromTop(img, x), whiteRunFromBottom(img, x); top <= bottom*2 {
		t.Errorf("Steg (%d px oben in den Druckdaten) ist nicht deutlich breiter als der Rand (%d px)", top, bottom)
	}
}

// TestRenderRejectsGarbage stellt sicher, dass eine kaputte Datei einen
// Fehler und keinen leeren Ausdruck erzeugt – Papier ist teuer.
func TestRenderRejectsGarbage(t *testing.T) {
	if _, err := printing.Render(bytes.NewReader([]byte("kein bild")), printing.TemplateFull, printing.NativeRaster4x6); err == nil {
		t.Fatal("erwartete einen Fehler für ungültige Bilddaten")
	}
}

// TestTemplateByIDFallsBack schützt vor dem Fall, dass die Oberfläche eine
// veraltete Layout-Auswahl schickt.
func TestTemplateByIDFallsBack(t *testing.T) {
	if got := printing.TemplateByID("gibt-es-nicht"); got.ID != printing.TemplateFull {
		t.Errorf("TemplateByID = %s, erwartet %s", got.ID, printing.TemplateFull)
	}
}

// isNearWhite toleriert die JPEG-Kompression, die auch reines Weiß leicht
// verfärbt.
func isNearWhite(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	const threshold = 0xF000

	return r > threshold && g > threshold && b > threshold
}

func whiteRunFromTop(img image.Image, x int) int {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		if !isNearWhite(img.At(x, y)) {
			return y - b.Min.Y
		}
	}

	return b.Dy()
}

func whiteRunFromBottom(img image.Image, x int) int {
	b := img.Bounds()
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		if !isNearWhite(img.At(x, y)) {
			return b.Max.Y - 1 - y
		}
	}

	return b.Dy()
}

// TestRenderWritesJFIFHeader sichert den Fehler ab, der dazu führte, dass
// Druckaufträge spurlos verschwanden.
//
// Gos image/jpeg schreibt kein JFIF-Segment; die Datei beginnt mit
// FF D8 FF DB. CUPS erkennt image/jpeg aber nur, wenn das vierte Byte ein
// Anwendungsmarker aus 0xE0 bis 0xEF ist (/usr/share/cups/mime/mime.types).
// Ohne das Segment meldet CUPS "The print file could not be opened", der
// Backend bricht mit Status 5 ab – und zwar ohne sichtbare Rückmeldung an der
// Fotobox, weil lp den Auftrag zuvor bereits angenommen hat.
func TestRenderWritesJFIFHeader(t *testing.T) {
	out, err := printing.Render(bytes.NewReader(loadSample(t)), printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if len(out) < 20 {
		t.Fatalf("Ergebnis ist mit %d Bytes zu kurz", len(out))
	}

	if out[0] != 0xFF || out[1] != 0xD8 {
		t.Fatalf("kein JPEG-Startmarker: % X", out[:2])
	}

	// Der von CUPS geprüfte Bereich.
	if out[2] != 0xFF || out[3] < 0xE0 || out[3] > 0xEF {
		t.Errorf("viertes Byte = 0x%02X, CUPS verlangt 0xE0 bis 0xEF – der Auftrag würde verworfen", out[3])
	}

	if got := string(out[6:10]); got != "JFIF" {
		t.Errorf("Segmentkennung = %q, erwartet %q", got, "JFIF")
	}

	// Auflösung, damit nachgelagerte Filter die 300 dpi nicht raten müssen.
	if unit := out[13]; unit != 1 {
		t.Errorf("Einheit = %d, erwartet 1 (Punkte pro Zoll)", unit)
	}

	x := int(out[14])<<8 | int(out[15])
	y := int(out[16])<<8 | int(out[17])
	if x != printing.DPI || y != printing.DPI {
		t.Errorf("Auflösung = %dx%d, erwartet %dx%d", x, y, printing.DPI, printing.DPI)
	}

	// Das Bild muss trotz des eingefügten Segments lesbar bleiben.
	if _, err := jpeg.Decode(bytes.NewReader(out)); err != nil {
		t.Errorf("Ergebnis ist nach dem Einfügen nicht mehr dekodierbar: %v", err)
	}
}

// TestRenderHonoursExifOrientation sichert ab, dass ein hochkant
// fotografiertes Motiv auch hochkant auf dem Papier landet.
//
// Kameras und Smartphones speichern das Bild in der Lage des Sensors und
// vermerken die tatsächliche Ausrichtung nur im EXIF-Block. Gos image/jpeg
// wertet ihn nicht aus. Ohne Korrektur legte der Renderer ein Hochformat auf
// die lange Papierkante, beschnitte es dort formatfüllend und der Ausdruck
// zeigte einen gedrehten, stark vergrößerten Ausschnitt.
func TestRenderHonoursExifOrientation(t *testing.T) {
	wantShort := printing.NativeRaster4x6.Short()
	wantLong := printing.NativeRaster4x6.Long()

	// Sensorlage quer, laut EXIF aber hochkant.
	src := jpegWithOrientation(t, 1200, 800, 6)

	out, err := printing.Render(bytes.NewReader(src), printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != wantShort || b.Dy() != wantLong {
		t.Errorf("Abmessungen = %dx%d, erwartet %dx%d – die EXIF-Ausrichtung wurde nicht beachtet",
			b.Dx(), b.Dy(), wantShort, wantLong)
	}
}

// TestRenderWithoutExifKeepsOrientation ist die Gegenprobe: Ohne Angabe darf
// nichts gedreht werden.
func TestRenderWithoutExifKeepsOrientation(t *testing.T) {
	wantShort := printing.NativeRaster4x6.Short()
	wantLong := printing.NativeRaster4x6.Long()

	src := jpegWithOrientation(t, 1200, 800, 0) // 0 = kein EXIF-Block

	out, err := printing.Render(bytes.NewReader(src), printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	b := img.Bounds()
	if b.Dx() != wantLong || b.Dy() != wantShort {
		t.Errorf("Abmessungen = %dx%d, erwartet %dx%d", b.Dx(), b.Dy(), wantLong, wantShort)
	}
}

// jpegWithOrientation erzeugt ein Testbild. Ist orientation 0, wird kein
// EXIF-Block geschrieben.
func jpegWithOrientation(t *testing.T, w, h, orientation int) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("cannot encode: %v", err)
	}

	raw := buf.Bytes()
	if orientation == 0 {
		return raw
	}

	tiff := []byte{
		'I', 'I', 42, 0,
		8, 0, 0, 0,
		1, 0,
		0x12, 0x01,
		3, 0,
		1, 0, 0, 0,
		byte(orientation), 0, 0, 0,
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

// TestRenderUsesNativeRasterSize hält den von Gutenprints PPD deklarierten
// Vertrag fest. Die Transportfläche enthält den randlosen Überstand und ist
// deshalb bewusst nicht 2:3; nur die sichtbare Medienfläche ist 1200x1800.
func TestRenderUsesNativeRasterSize(t *testing.T) {
	if printing.NativeRaster4x6.Width != 1266 || printing.NativeRaster4x6.Height != 1836 {
		t.Fatalf("NativeRaster4x6 = %dx%d, PPD deklariert 1266x1836",
			printing.NativeRaster4x6.Width, printing.NativeRaster4x6.Height)
	}
	if printing.VisibleMedia4x6.Width != 1200 || printing.VisibleMedia4x6.Height != 1800 {
		t.Fatalf("VisibleMedia4x6 = %dx%d, erwartet 1200x1800",
			printing.VisibleMedia4x6.Width, printing.VisibleMedia4x6.Height)
	}

	out, err := printing.Render(bytes.NewReader(loadSample(t)), printing.TemplateFull, printing.NativeRaster4x6)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	img, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("Ergebnis ist kein gültiges JPEG: %v", err)
	}

	if b := img.Bounds(); b.Dx() != 1266 || b.Dy() != 1836 {
		t.Errorf("Abmessungen = %dx%d, erwartet 1266x1836", b.Dx(), b.Dy())
	}
}

func TestRenderRejectsNonNativeRaster(t *testing.T) {
	for _, raster := range []printing.Raster{
		{},
		{Width: 1266},
		{Height: 1836},
		{Width: -1, Height: -1},
		{Width: 1224, Height: 1836},
	} {
		if _, err := printing.Render(bytes.NewReader(loadSample(t)), printing.TemplateFull, raster); err == nil {
			t.Errorf("Render(%v) akzeptierte einen falschen Raster", raster)
		}
	}
}
