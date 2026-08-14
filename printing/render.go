package printing

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"

	xdraw "golang.org/x/image/draw"

	"github.com/torbenschinke/eventprint/orient"
)

const (
	// DPI ist die Auflösung des Druckers.
	//
	// Der CZ-01 beherrscht ausschließlich 300x300 dpi – anders als
	// verwandte Modelle wie CW-02, CX-02 oder DNP DS620, für die Gutenprint
	// zusätzlich 300x600 dpi anbietet. Es gibt hier also keine höhere Stufe,
	// die sich wählen ließe.
	DPI = 300

	// CupsPageSize ist die PPD-Bezeichnung für 4x6" (288x432 Punkte à 1/72").
	CupsPageSize = "w288h432"
)

// Raster ist die Pixelgröße, in der der Drucker eine Seite tatsächlich
// erwartet.
type Raster struct {
	Width  int
	Height int
}

// Short liefert die kurze, Long die lange Kante.
func (r Raster) Short() int { return min(r.Width, r.Height) }
func (r Raster) Long() int  { return max(r.Width, r.Height) }

// Valid meldet, ob die Angabe brauchbar ist.
func (r Raster) Valid() bool { return r.Width > 0 && r.Height > 0 }

// NativeRaster4x6 ist die Rastergröße, die der CZ-01 für 10x15 cm erwartet.
//
// Der Wert ist bewusst nicht 1200x1800 (also 4x6 Zoll mal 300 dpi), sondern
// etwas größer: Dye-Sublimation-Drucker drucken randlos und benötigen dafür
// einen Überstand. Der Treiber verlangt exakt 1224x1836 Pixel, was 4,08 x
// 6,12 Zoll entspricht; das Seitenverhältnis von 2:3 bleibt dabei erhalten.
//
// Wird in einer anderen Größe gerendert, skaliert die CUPS-Filterkette das
// Bild auf diesen Wert hoch und weicht es dabei auf. Der richtige Wert für
// eine andere Papiersorte oder ein anderes Modell lässt sich ablesen mit
//
//	cupsctl --debug-logging
//	# einmal drucken, dann:
//	grep -a "cupsWidth\|cupsHeight" /var/log/cups/error_log
var NativeRaster4x6 = Raster{Width: 1224, Height: 1836}

// jpegQuality ist bewusst hoch gewählt: das Bild wird nur einmal, direkt vor
// dem Druck, neu komprimiert.
const jpegQuality = 95

// Render dekodiert das Quellbild aus src und erzeugt eine druckfertige
// JPEG-Datei in genau der Pixelgröße, die der Drucker erwartet.
//
// Die Ausrichtung (Hoch- oder Querformat) wird aus dem Quellbild übernommen,
// sodass ein hochkant fotografiertes Motiv auch hochkant gedruckt wird.
//
// Ist raster ungültig, wird [NativeRaster4x6] verwendet.
func Render(src io.Reader, tpl TemplateID, raster Raster) ([]byte, error) {
	raw, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("cannot read source image: %w", err)
	}

	// Die Ausrichtung wird hier erneut ausgewertet, obwohl der Import sie
	// bereits normalisiert: Fotos, die vor dieser Korrektur gespeichert
	// wurden, kämen sonst weiterhin gedreht aus dem Drucker. Bei bereits
	// aufgerichteten Bildern kostet die Prüfung nichts.
	img, _, err := orient.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("cannot decode source image: %w", err)
	}

	canvas := renderTemplate(img, tpl, raster)

	applyCalibration(canvas)
	compensateDriverRotation(canvas)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("cannot encode print image: %w", err)
	}

	return withJFIFHeader(buf.Bytes()), nil
}

// applyCalibration legt die am Gerät ausgemessene Kennlinie des
// Herstellertreibers auf die fertige Seite. Sie ist Teil des Renderings und
// gilt damit sowohl für den bevorzugten Custom-Filter als auch für den
// Systemtreiber-Fallback. Der Custom-Pfad reicht den danach erzeugten
// Druckstrom raw an CUPS weiter; die Kurve wird dort nicht erneut angewendet.
//
// Gutenprint reicht die Werte für den CZ-01 unverändert durch, der
// Herstellertreiber nicht. Ohne diesen Ausgleich trägt der Drucker in dunklen
// Partien zu viel Farbe auf, die dann in Transportrichtung verläuft. Siehe
// calibration.go.
func applyCalibration(img *image.RGBA) {
	c := printerCalibration

	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0] = c.red[img.Pix[i+0]]
		img.Pix[i+1] = c.green[img.Pix[i+1]]
		img.Pix[i+2] = c.blue[img.Pix[i+2]]
	}
}

// compensateDriverRotation dreht die Seite um 180 Grad.
//
// Das gleicht einen Fehler in Gutenprint aus: Für den QW410/CZ-01 ist das
// Merkmal DYESUB_FEATURE_PLANE_LEFTTORIGHT gesetzt, was jede Bildzeile
// spiegelt (print-dyesub.c: "col = pv->imgw_px - col - 1"), und zusätzlich
// werden die Zeilen von oben nach unten in ein BMP geschrieben, dessen
// positive Höhe die umgekehrte Reihenfolge bedeutet. Zusammen ergibt das eine
// Drehung um 180 Grad, die der Herstellertreiber nicht vornimmt.
//
// Sollte Gutenprint das eines Tages beheben, muss diese Kompensation
// entfallen — der Test TestCompensateDriverRotation hält fest, worauf zu
// achten ist.
func compensateDriverRotation(img *image.RGBA) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	// Punktweise tauschen: (x,y) gegen (w-1-x, h-1-y). Die Schleife läuft nur
	// über die halbe Seite, sonst würde jeder Tausch wieder rückgängig
	// gemacht.
	for y := range h / 2 {
		for x := range w {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			j := img.PixOffset(b.Min.X+w-1-x, b.Min.Y+h-1-y)

			for k := range 4 {
				img.Pix[i+k], img.Pix[j+k] = img.Pix[j+k], img.Pix[i+k]
			}
		}
	}

	// Bei ungerader Höhe bleibt die mittlere Zeile übrig; sie muss noch
	// gespiegelt werden.
	if h%2 == 1 {
		y := h / 2
		for x := range w / 2 {
			i := img.PixOffset(b.Min.X+x, b.Min.Y+y)
			j := img.PixOffset(b.Min.X+w-1-x, b.Min.Y+y)

			for k := range 4 {
				img.Pix[i+k], img.Pix[j+k] = img.Pix[j+k], img.Pix[i+k]
			}
		}
	}
}

// jfifHeader ist ein JFIF-APP0-Segment mit der Auflösung des Druckers.
//
//	FF E0        Marker APP0
//	00 10        Segmentlänge 16
//	4A 46 49 46 00  "JFIF\0"
//	01 01        Version 1.01
//	01           Einheit: Punkte pro Zoll
//	01 2C 01 2C  300 x 300 dpi
//	00 00        kein Vorschaubild
var jfifHeader = []byte{
	0xFF, 0xE0,
	0x00, 0x10,
	0x4A, 0x46, 0x49, 0x46, 0x00,
	0x01, 0x01,
	0x01,
	byte(DPI >> 8), byte(DPI & 0xFF),
	byte(DPI >> 8), byte(DPI & 0xFF),
	0x00, 0x00,
}

// withJFIFHeader ergänzt das von Go erzeugte JPEG um ein JFIF-Segment.
//
// Das ist keine Kosmetik, sondern zwingend notwendig: Gos image/jpeg schreibt
// hinter den Startmarker direkt die Quantisierungstabelle, die Datei beginnt
// also mit FF D8 FF DB. CUPS erkennt eine Datei aber nur dann als image/jpeg,
// wenn das vierte Byte ein Anwendungsmarker aus dem Bereich 0xE0 bis 0xEF ist
// (siehe /usr/share/cups/mime/mime.types). Ohne dieses Segment kann CUPS den
// Dateityp nicht bestimmen, bricht mit "The print file could not be opened"
// ab und der Auftrag verschwindet, ohne dass etwas gedruckt wird.
//
// Nebenbei trägt das Segment die Auflösung ein, sodass nachgelagerte Filter
// die 300 dpi nicht raten müssen.
func withJFIFHeader(jpg []byte) []byte {
	const (
		soiLen       = 2    // FF D8
		appMarkerLow = 0xE0 // erster Anwendungsmarker
		appMarkerTop = 0xEF // letzter Anwendungsmarker
	)

	// Zu kurz, um ein JPEG zu sein: unverändert zurückgeben, damit der Fehler
	// dort auffällt, wo er entsteht.
	if len(jpg) < soiLen+2 {
		return jpg
	}

	// Bereits ein Anwendungssegment vorhanden, etwa weil eine spätere
	// Go-Version JFIF selbst schreibt.
	if jpg[2] == 0xFF && jpg[3] >= appMarkerLow && jpg[3] <= appMarkerTop {
		return jpg
	}

	out := make([]byte, 0, len(jpg)+len(jfifHeader))
	out = append(out, jpg[:soiLen]...)
	out = append(out, jfifHeader...)
	out = append(out, jpg[soiLen:]...)

	return out
}

// renderTemplate legt das Motiv gemäß Layout auf eine weiße Seite.
func renderTemplate(img image.Image, tpl TemplateID, raster Raster) *image.RGBA {
	if !raster.Valid() {
		raster = NativeRaster4x6
	}

	bounds := img.Bounds()

	pageW, pageH := raster.Short(), raster.Long()
	if bounds.Dx() >= bounds.Dy() {
		pageW, pageH = raster.Long(), raster.Short()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, pageW, pageH))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	switch tpl {
	case TemplateBorder:
		// gleichmäßiger Rand von 5 % der kurzen Seite, das Motiv wird
		// vollständig eingepasst (contain).
		margin := min(pageW, pageH) * 5 / 100
		area := image.Rect(margin, margin, pageW-margin, pageH-margin)
		drawContain(canvas, area, img)

	case TemplatePolaroid:
		// klassische Sofortbild-Proportionen: schmaler Rand oben/seitlich,
		// breiter Steg unten zum Beschriften.
		side := min(pageW, pageH) * 6 / 100
		bottom := min(pageW, pageH) * 22 / 100
		area := image.Rect(side, side, pageW-side, pageH-bottom)
		drawCover(canvas, area, img)

	default: // TemplateFull
		drawCover(canvas, canvas.Bounds(), img)
	}

	return canvas
}

// drawCover skaliert das Motiv formatfüllend in area und beschneidet mittig,
// was übersteht. Das entspricht CSS object-fit: cover.
func drawCover(dst *image.RGBA, area image.Rectangle, src image.Image) {
	sb := src.Bounds()
	if sb.Empty() || area.Empty() {
		return
	}

	scale := max(
		float64(area.Dx())/float64(sb.Dx()),
		float64(area.Dy())/float64(sb.Dy()),
	)

	// Ausschnitt der Quelle, der nach dem Skalieren exakt area ausfüllt.
	cropW := int(float64(area.Dx()) / scale)
	cropH := int(float64(area.Dy()) / scale)
	cropW = min(cropW, sb.Dx())
	cropH = min(cropH, sb.Dy())

	offX := sb.Min.X + (sb.Dx()-cropW)/2
	offY := sb.Min.Y + (sb.Dy()-cropH)/2
	crop := image.Rect(offX, offY, offX+cropW, offY+cropH)

	xdraw.CatmullRom.Scale(dst, area, src, crop, draw.Over, nil)
}

// drawContain skaliert das Motiv so, dass es vollständig in area passt, und
// zentriert es dort. Das entspricht CSS object-fit: contain.
func drawContain(dst *image.RGBA, area image.Rectangle, src image.Image) {
	sb := src.Bounds()
	if sb.Empty() || area.Empty() {
		return
	}

	scale := min(
		float64(area.Dx())/float64(sb.Dx()),
		float64(area.Dy())/float64(sb.Dy()),
	)

	w := int(float64(sb.Dx()) * scale)
	h := int(float64(sb.Dy()) * scale)

	offX := area.Min.X + (area.Dx()-w)/2
	offY := area.Min.Y + (area.Dy()-h)/2
	target := image.Rect(offX, offY, offX+w, offY+h)

	xdraw.CatmullRom.Scale(dst, target, src, sb, draw.Over, nil)
}
