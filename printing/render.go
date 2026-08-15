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

// Matches akzeptiert Hoch- und Querformat derselben Rastergröße.
func (r Raster) Matches(width, height int) bool {
	return (width == r.Width && height == r.Height) ||
		(width == r.Height && height == r.Width)
}

// NativeRaster4x6 ist die Bildfläche, die Gutenprint für 4x6 auf dem CZ-01
// über seine PPD deklariert.
//
// 1266x1836 enthält den randlosen Überstand um das sichtbare 1200x1800-Papier.
// Andere Größen werden vor dem Spooling abgelehnt, damit Gutenprint niemals
// selbst per Point-Sampling skaliert.
var NativeRaster4x6 = Raster{Width: 1266, Height: 1836}

// VisibleMedia4x6 ist die physisch sichtbare 4x6-Fläche bei 300 dpi.
var VisibleMedia4x6 = Raster{Width: 1200, Height: 1800}

// jpegQuality ist bewusst hoch gewählt: das Bild wird nur einmal, direkt vor
// dem Druck, neu komprimiert.
const jpegQuality = 95

// Render dekodiert das Quellbild aus src und erzeugt eine druckfertige
// JPEG-Datei in genau der Pixelgröße, die der Drucker erwartet.
//
// Die Ausrichtung (Hoch- oder Querformat) wird aus dem Quellbild übernommen,
// sodass ein hochkant fotografiertes Motiv auch hochkant gedruckt wird.
//
// Ausschließlich [NativeRaster4x6] ist zulässig. Dadurch kann kein Aufrufer
// versehentlich eine Größe erzeugen, die Gutenprint später skaliert.
func Render(src io.Reader, tpl TemplateID, raster Raster) ([]byte, error) {
	if raster != NativeRaster4x6 {
		return nil, fmt.Errorf("unsupported print raster %dx%d: expected %dx%d",
			raster.Width, raster.Height, NativeRaster4x6.Width, NativeRaster4x6.Height)
	}
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
// Herstellertreibers auf die fertige Seite. Die nachfolgende Rasterpipeline
// reicht diese Werte unverändert an Gutenprint weiter.
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
	bounds := img.Bounds()

	pageW, pageH := raster.Short(), raster.Long()
	// Ein Polaroid hat unabhängig von der Motivausrichtung eine feste
	// Hochformat-Geometrie. Würde die Seite bei einem Querformatmotiv gedreht,
	// läge der breite Steg an einer langen statt an derselben kurzen Kante, die
	// auch die Vorschau zeigt.
	if tpl == TemplateBorder && borderFitsBetter(bounds, raster.Long(), raster.Short()) {
		pageW, pageH = raster.Long(), raster.Short()
	} else if tpl != TemplatePolaroid && tpl != TemplateBorder && bounds.Dx() >= bounds.Dy() {
		pageW, pageH = raster.Long(), raster.Short()
	}

	canvas := image.NewRGBA(image.Rect(0, 0, pageW, pageH))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	visibleW, visibleH := VisibleMedia4x6.Short(), VisibleMedia4x6.Long()
	if pageW > pageH {
		visibleW, visibleH = visibleH, visibleW
	}
	visible := image.Rect(
		(pageW-visibleW)/2,
		(pageH-visibleH)/2,
		(pageW-visibleW)/2+visibleW,
		(pageH-visibleH)/2+visibleH,
	)

	switch tpl {
	case TemplateBorder:
		// gleichmäßiger Rand von 5 % der kurzen Seite, das Motiv wird
		// vollständig eingepasst (contain).
		margin := min(visibleW, visibleH) * 5 / 100
		area := image.Rect(visible.Min.X+margin, visible.Min.Y+margin, visible.Max.X-margin, visible.Max.Y-margin)
		drawContain(canvas, area, img)

	case TemplatePolaroid:
		// klassische Sofortbild-Proportionen: schmaler Rand oben/seitlich,
		// breiter Steg unten zum Beschriften.
		side := min(visibleW, visibleH) * 6 / 100
		bottom := min(visibleW, visibleH) * 22 / 100
		area := image.Rect(visible.Min.X+side, visible.Min.Y+side, visible.Max.X-side, visible.Max.Y-bottom)
		drawCover(canvas, area, img)

	default: // TemplateFull
		drawCover(canvas, canvas.Bounds(), img)
	}

	return canvas
}

// borderFitsBetter vergleicht die tatsächlich nutzbare Motivgröße in beiden
// Papierorientierungen. Das ist robuster als nur Hoch- und Querformat des
// Originals zu vergleichen und garantiert auch für Quadrat-, Panorama- und
// andere Seitenverhältnisse die größtmögliche vollständige Abbildung.
func borderFitsBetter(src image.Rectangle, pageW, pageH int) bool {
	landscape := borderContainScale(src, pageW, pageH)
	portrait := borderContainScale(src, pageH, pageW)
	return landscape > portrait
}

func borderContainScale(src image.Rectangle, pageW, pageH int) float64 {
	visibleW, visibleH := VisibleMedia4x6.Short(), VisibleMedia4x6.Long()
	if pageW > pageH {
		visibleW, visibleH = visibleH, visibleW
	}
	margin := min(visibleW, visibleH) * 5 / 100
	usableW, usableH := visibleW-2*margin, visibleH-2*margin
	return min(float64(usableW)/float64(src.Dx()), float64(usableH)/float64(src.Dy()))
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
