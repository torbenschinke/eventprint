package printing

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSystemGutenprintReceivesExactCZ01Raster(t *testing.T) {
	if _, err := os.Stat("/etc/cups/ppd/CZ01.ppd"); err != nil {
		t.Skip("CZ01 PPD nicht installiert")
	}
	if _, err := os.Stat("/usr/lib/cups/filter/rastertogutenprint.5.3"); err != nil {
		t.Skip("System-Gutenprint nicht installiert")
	}

	sample, err := os.ReadFile("../DSC02301.jpg")
	if err != nil {
		t.Skipf("Beispielbild nicht vorhanden: %v", err)
	}
	jpegData, err := Render(bytes.NewReader(sample), TemplateFull, NativeRaster4x6)
	if err != nil {
		t.Fatal(err)
	}
	jpegFile := filepath.Join(t.TempDir(), "print.jpg")
	if err := os.WriteFile(jpegFile, jpegData, 0o600); err != nil {
		t.Fatal(err)
	}

	stream, err := (CUPSPrinter{Queue: "CZ01"}).filterJPEG(context.Background(), jpegFile, "proof")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stream)

	data, err := os.ReadFile(stream)
	if err != nil {
		t.Fatal(err)
	}
	for _, plane := range []string{"YPLANE", "MPLANE", "CPLANE"} {
		marker := []byte("\x1bPIMAGE " + plane)
		i := bytes.Index(data, marker)
		if i < 0 {
			t.Fatalf("%s fehlt im Druckstrom", plane)
		}
		length, err := strconv.Atoi(string(data[i+24 : i+32]))
		if err != nil {
			t.Fatal(err)
		}
		bmp := data[i+32 : i+32+length]
		if got := int(binary.LittleEndian.Uint32(bmp[18:22])); got != 1408 {
			t.Errorf("%s BMP-Breite = %d, erwartet 1408", plane, got)
		}
		if got := int(binary.LittleEndian.Uint32(bmp[22:26])); got != 1836 {
			t.Errorf("%s BMP-Höhe = %d, erwartet 1836", plane, got)
		}
	}
}

func TestSystemGutenprintDoesNotResample1266Input(t *testing.T) {
	if _, err := os.Stat("/etc/cups/ppd/CZ01.ppd"); err != nil {
		t.Skip("CZ01 PPD nicht installiert")
	}
	jpegFile := filepath.Join(t.TempDir(), "black.jpg")
	if err := os.WriteFile(jpegFile, solidJPEG(t, 1266, 1836, color.Black), 0o600); err != nil {
		t.Fatal(err)
	}
	stream, err := (CUPSPrinter{Queue: "CZ01"}).filterJPEG(context.Background(), jpegFile, "proof")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(stream)
	data, err := os.ReadFile(stream)
	if err != nil {
		t.Fatal(err)
	}

	for _, plane := range []string{"YPLANE", "MPLANE", "CPLANE"} {
		i := bytes.Index(data, []byte("\x1bPIMAGE "+plane))
		length, _ := strconv.Atoi(string(data[i+24 : i+32]))
		bmp := data[i+32 : i+32+length]
		offset := int(binary.LittleEndian.Uint32(bmp[10:14]))
		if offset == 0 {
			offset = 1088
		}
		row := bmp[offset+900*1408 : offset+901*1408]
		active := make([]int, 0, 1266)
		for x, value := range row {
			if value < 250 {
				active = append(active, x)
			}
		}
		// Debian's 5.3.4 CUPS adapter truncates the PPD's floating-point
		// width to 1265 before handing it to print-dyesub. Since the source
		// has 1266 columns, point interpolation maps each output column to a
		// distinct source column and drops only one outermost bleed column. No
		// interior pixel is duplicated or skipped.
		if len(active) != 1265 || active[0] != 71 || active[len(active)-1] != 1335 {
			t.Errorf("%s aktive Pixel = %d, Bereich %d..%d; erwartet 1265, 71..1335",
				plane, len(active), active[0], active[len(active)-1])
		}
	}
}

func TestValidateCZ01PPDRejectsWrongImageableArea(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CZ01.ppd")
	valid := "*ImageableArea w288h432/4x6: \"17.040 0.000 320.880 440.640\"\n*Resolution 300dpi/300x300 DPI: \"\"\n"
	if err := os.WriteFile(path, []byte(valid), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateCZ01PPD(path); err != nil {
		t.Fatalf("gültige PPD wurde abgelehnt: %v", err)
	}
	wrong := strings.Replace(valid, "320.880", "315.840", 1)
	if err := os.WriteFile(path, []byte(wrong), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateCZ01PPD(path); err == nil {
		t.Fatal("falsche ImageableArea wurde nicht erkannt")
	}
}

func TestWriteCZ01RasterProducesExactContract(t *testing.T) {
	for _, landscape := range []bool{false, true} {
		t.Run(map[bool]string{false: "portrait", true: "landscape"}[landscape], func(t *testing.T) {
			w, h := NativeRaster4x6.Width, NativeRaster4x6.Height
			if landscape {
				w, h = h, w
			}
			jpegData := solidJPEG(t, w, h, color.RGBA{R: 20, G: 40, B: 60, A: 255})

			var raster bytes.Buffer
			if err := writeCZ01Raster(&raster, jpegData); err != nil {
				t.Fatal(err)
			}
			header, err := readCUPSRasterHeader(bytes.NewReader(raster.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			if header.Width != 1266 || header.Height != 1836 || header.DPIX != 300 || header.DPIY != 300 ||
				header.BytesPerLine != 3798 || header.BitsPerPixel != 24 || header.ColorSpace != 1 {
				t.Fatalf("unerwarteter Rasterheader: %+v", header)
			}
			if got, want := raster.Len(), cupsRasterMagicSize+cupsRasterHeaderSize+1266*1836*3; got != want {
				t.Fatalf("Rasterlänge = %d, erwartet %d", got, want)
			}
		})
	}
}

func TestWriteCZ01RasterRejectsWrongImageSize(t *testing.T) {
	jpegData := solidJPEG(t, 1224, 1836, color.Black)
	if err := writeCZ01Raster(&bytes.Buffer{}, jpegData); err == nil {
		t.Fatal("1224x1836 muss vor Gutenprint abgelehnt werden")
	}
}

func TestValidateCZ01RasterRejectsCorruptHeader(t *testing.T) {
	jpegData := solidJPEG(t, 1266, 1836, color.Black)
	path := filepath.Join(t.TempDir(), "job.raster")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeCZ01Raster(f, jpegData); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := validateCZ01Raster(path); err != nil {
		t.Fatalf("gültiger Raster wurde abgelehnt: %v", err)
	}

	f, err = os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteAt([]byte{0, 0, 0, 0}, cupsRasterMagicSize+372); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if err := validateCZ01Raster(path); err == nil {
		t.Fatal("beschädigte Rasterbreite wurde nicht erkannt")
	}
}

func TestTemplatesUseVisibleMediaPixels(t *testing.T) {
	border := renderTemplate(solidImage(1080, 1680, color.Black), TemplateBorder, NativeRaster4x6, RenderOptions{})
	assertTransition(t, border, 93, 78, 1173, 1758)

	polaroid := renderTemplate(solidImage(1056, 1464, color.Black), TemplatePolaroid, NativeRaster4x6, RenderOptions{})
	assertTransition(t, polaroid, 105, 90, 1161, 1554)
}

func TestBorderChoosesBestOrientationAndCentersEntireImage(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		height    int
		landscape bool
	}{
		{name: "panorama", width: 2400, height: 400, landscape: true},
		{name: "portrait", width: 400, height: 2400, landscape: false},
		{name: "square", width: 1000, height: 1000, landscape: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := renderTemplate(solidImage(tt.width, tt.height, color.Black), TemplateBorder, NativeRaster4x6, RenderOptions{})
			bounds := img.Bounds()
			if got := bounds.Dx() > bounds.Dy(); got != tt.landscape {
				t.Fatalf("Papier quer = %v, erwartet %v (%dx%d)", got, tt.landscape, bounds.Dx(), bounds.Dy())
			}

			motif := nonWhiteBounds(img)
			if motif.Empty() {
				t.Fatal("Motiv fehlt")
			}
			left, right := motif.Min.X-bounds.Min.X, bounds.Max.X-motif.Max.X
			top, bottom := motif.Min.Y-bounds.Min.Y, bounds.Max.Y-motif.Max.Y
			if abs(left-right) > 1 || abs(top-bottom) > 1 {
				t.Errorf("Motiv nicht zentriert: links=%d rechts=%d oben=%d unten=%d", left, right, top, bottom)
			}

			gotRatio := float64(motif.Dx()) / float64(motif.Dy())
			wantRatio := float64(tt.width) / float64(tt.height)
			if difference := gotRatio/wantRatio - 1; difference < -0.01 || difference > 0.01 {
				t.Errorf("Motivseitenverhältnis %.4f, erwartet %.4f; Bild wurde beschnitten", gotRatio, wantRatio)
			}
		})
	}
}

func nonWhiteBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.RGBAModel.Convert(img.At(x, y)).(color.RGBA) == (color.RGBA{255, 255, 255, 255}) {
				continue
			}
			minX, minY = min(minX, x), min(minY, y)
			maxX, maxY = max(maxX, x+1), max(maxY, y+1)
		}
	}
	return image.Rect(minX, minY, maxX, maxY)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func assertTransition(t *testing.T, img image.Image, left, top, right, bottom int) {
	t.Helper()
	centerX := (left + right) / 2
	centerY := (top + bottom) / 2
	for _, p := range []image.Point{{left - 1, centerY}, {centerX, top - 1}, {right, centerY}, {centerX, bottom}} {
		if c := color.RGBAModel.Convert(img.At(p.X, p.Y)).(color.RGBA); c != (color.RGBA{255, 255, 255, 255}) {
			t.Errorf("Pixel vor Übergang %v ist %v statt weiß", p, c)
		}
	}
	for _, p := range []image.Point{{left, centerY}, {centerX, top}, {right - 1, centerY}, {centerX, bottom - 1}} {
		if c := color.RGBAModel.Convert(img.At(p.X, p.Y)).(color.RGBA); c != (color.RGBA{0, 0, 0, 255}) {
			t.Errorf("Pixel im Motiv %v ist %v statt schwarz", p, c)
		}
	}
}

func solidJPEG(t *testing.T, width, height int, c color.Color) []byte {
	t.Helper()
	img := solidImage(width, height, c)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func solidImage(width, height int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, c)
		}
	}
	return img
}

// TestPaperLandscapeMatchesRenderedPage bindet die Regel, nach der die
// Vorschau das Papier dreht, an die tatsächlich erzeugte Seite.
//
// Die Vorschau hat die Drehung früher gar nicht nachvollzogen und zeigte
// immer ein hochkantes Blatt. Ein Querformatfoto sah dort deshalb völlig
// anders aus als der Ausdruck. Seitdem beide dieselbe Funktion befragen, muss
// dieser Test sicherstellen, dass sie nicht wieder auseinanderlaufen.
func TestPaperLandscapeMatchesRenderedPage(t *testing.T) {
	sizes := []struct {
		name          string
		width, height int
	}{
		{"panorama", 2400, 400},
		{"quer 3:2", 3000, 2000},
		{"quer 4:3", 2048, 1536},
		{"quadrat", 1000, 1000},
		{"hoch 3:4", 1536, 2048},
		{"hoch schmal", 400, 2400},
	}

	for _, tpl := range Templates() {
		for _, size := range sizes {
			t.Run(string(tpl.ID)+"/"+size.name, func(t *testing.T) {
				page := renderTemplate(solidImage(size.width, size.height, color.Black), tpl.ID, NativeRaster4x6, RenderOptions{})

				rendered := page.Bounds().Dx() > page.Bounds().Dy()
				predicted := PaperLandscape(tpl.ID, size.width, size.height)

				if rendered != predicted {
					t.Fatalf("PaperLandscape = %v, gedruckt wird quer = %v", predicted, rendered)
				}
			})
		}
	}
}

// TestPaperLandscapeWithoutDimensions deckt den Zustand ab, in dem die
// Vorschau noch kein Bild kennt. Ohne Maße darf sie kein Querformat raten.
func TestPaperLandscapeWithoutDimensions(t *testing.T) {
	for _, tpl := range Templates() {
		for _, size := range [][2]int{{0, 0}, {0, 100}, {100, 0}, {-1, -1}} {
			if PaperLandscape(tpl.ID, size[0], size[1]) {
				t.Errorf("%s bei %dx%d quer, erwartet hochkant", tpl.ID, size[0], size[1])
			}
		}
	}
}
