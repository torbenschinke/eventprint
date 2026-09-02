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

// FaceDetector liefert Gesichtsrechtecke in den Koordinaten des übergebenen
// Bildes. Fehler werden als leere Trefferliste behandelt, damit ein Ausdruck
// nie an der optionalen Erkennung scheitert.
type FaceDetector func(image.Image) []image.Rectangle

// RenderOptions steuert optionale, layoutabhängige Bildkorrekturen.
type RenderOptions struct {
	// AutoCrop richtet den Polaroid-Ausschnitt an erkannten Gesichtern aus.
	AutoCrop bool

	// DetectFaces wird von außen gesetzt, damit dieses Paket unabhängig von
	// OpenCV bleibt und der Upload-Service weiterhin ohne cgo baut.
	DetectFaces FaceDetector
}

// orDefaultRenderOptions macht die Angabe optional: nil bedeutet "keine
// Bildkorrekturen".
func orDefaultRenderOptions(load func() RenderOptions) func() RenderOptions {
	if load != nil {
		return load
	}

	return func() RenderOptions { return RenderOptions{} }
}

// Render dekodiert das Quellbild aus src und erzeugt eine druckfertige
// JPEG-Datei in genau der Pixelgröße, die der Drucker erwartet.
//
// Die Ausrichtung (Hoch- oder Querformat) wird aus dem Quellbild übernommen,
// sodass ein hochkant fotografiertes Motiv auch hochkant gedruckt wird.
//
// Ausschließlich [NativeRaster4x6] ist zulässig. Dadurch kann kein Aufrufer
// versehentlich eine Größe erzeugen, die Gutenprint später skaliert.
func Render(src io.Reader, tpl TemplateID, raster Raster) ([]byte, error) {
	return RenderWithOptions(src, tpl, raster, RenderOptions{})
}

// RenderWithOptions rendert wie [Render] und berücksichtigt zusätzlich die
// übergebenen Bildkorrekturen.
func RenderWithOptions(src io.Reader, tpl TemplateID, raster Raster, opts RenderOptions) ([]byte, error) {
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

	canvas := renderTemplate(img, tpl, raster, opts)

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

// PaperLandscape meldet, ob der Ausdruck im Querformat entsteht.
//
// Die Fotobox dreht das Papier, damit das Motiv so groß wie möglich
// herauskommt. Die Vorschau muss dieselbe Entscheidung treffen, sonst zeigt
// sie bei einem Querformatfoto ein hochkantes Blatt und damit einen ganz
// anderen Ausschnitt als der Ausdruck. Deshalb steht die Regel hier an einer
// einzigen Stelle und nicht zweimal nebeneinander.
//
// Ein Polaroid behält immer das Hochformat: Der breite Steg gehört an die
// kurze Kante, sonst ist es kein Sofortbild mehr.
func PaperLandscape(tpl TemplateID, imgW, imgH int) bool {
	// Unbekannte Maße: Die Vorschau fragt schon, bevor ein Bild gewählt ist.
	// Hochkant ist dann die richtige Annahme, weil ein leeres Blatt so
	// aussieht wie die Papierschublade des Druckers.
	if imgW <= 0 || imgH <= 0 {
		return false
	}

	switch tpl {
	case TemplatePolaroid:
		return false

	case TemplateBorder:
		// Bei quadratischen Motiven ist keine Richtung im Vorteil; das
		// Hochformat bleibt dann die ruhigere Wahl.
		return borderFitsBetter(image.Rect(0, 0, imgW, imgH), NativeRaster4x6.Long(), NativeRaster4x6.Short())

	default:
		return imgW >= imgH
	}
}

// renderTemplate legt das Motiv gemäß Layout auf eine weiße Seite.
func renderTemplate(img image.Image, tpl TemplateID, raster Raster, opts RenderOptions) *image.RGBA {
	bounds := img.Bounds()

	pageW, pageH := raster.Short(), raster.Long()
	if PaperLandscape(tpl, bounds.Dx(), bounds.Dy()) {
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
		// Gleichmäßiger Einzug von 5 % der kurzen Seite, danach wird das Motiv
		// vollständig eingepasst (contain).
		//
		// Der Einzug ist auf allen vier Seiten gleich, der sichtbare weiße
		// Rand ist es nicht: Das eingepasste Motiv behält sein
		// Seitenverhältnis, und was an den beiden übrigen Kanten frei bleibt,
		// kommt dort zum Einzug hinzu. Bei einem 4:3-Foto ist der Rand seitlich
		// rund dreimal so breit wie oben und unten.
		margin := min(visibleW, visibleH) * 5 / 100
		area := image.Rect(visible.Min.X+margin, visible.Min.Y+margin, visible.Max.X-margin, visible.Max.Y-margin)
		drawContain(canvas, area, img)

	case TemplatePolaroid:
		// klassische Sofortbild-Proportionen: schmaler Rand oben/seitlich,
		// breiter Steg unten zum Beschriften.
		side := min(visibleW, visibleH) * 6 / 100
		bottom := min(visibleW, visibleH) * 22 / 100
		area := image.Rect(visible.Min.X+side, visible.Min.Y+side, visible.Max.X-side, visible.Max.Y-bottom)
		if opts.AutoCrop && opts.DetectFaces != nil {
			drawCoverCrop(canvas, area, img, cropForFaces(img.Bounds(), opts.DetectFaces(img), area))
		} else {
			drawCover(canvas, area, img)
		}

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

	drawCoverCrop(dst, area, src, crop)
}

func drawCoverCrop(dst *image.RGBA, area image.Rectangle, src image.Image, crop image.Rectangle) {
	if crop.Empty() {
		drawCover(dst, area, src)
		return
	}
	xdraw.CatmullRom.Scale(dst, area, src, crop, draw.Over, nil)
}

// faceGroup spannt eine gemeinsame Box über alle erkannten Gesichter auf.
// Treffer außerhalb des Bildes werden beschnitten, damit ein Detektor mit
// leicht überstehenden Rechtecken die Box nicht künstlich aufbläht.
func faceGroup(bounds image.Rectangle, faces []image.Rectangle) image.Rectangle {
	var group image.Rectangle
	for _, face := range faces {
		face = face.Intersect(bounds)
		if face.Empty() {
			continue
		}
		if group.Empty() {
			group = face
		} else {
			group = group.Union(face)
		}
	}

	return group
}

// cropForFaces legt die gemeinsame Gesichtsbox in die Mitte der oberen
// Hälfte des Ausschnitts. Links, rechts und oben bleiben nach Möglichkeit
// jeweils 10 % der Ausschnitthöhe frei.
//
// Der gelieferte Ausschnitt hat exakt das Seitenverhältnis von area, denn
// [drawCoverCrop] bildet ihn ohne Rücksicht auf Proportionen darauf ab – jede
// Abweichung würde das Motiv verzerren.
//
// Nicht immer sind alle Vorgaben erfüllbar: Ein Ausschnitt kann das Original
// nicht überschreiten, und angeschnittene oder sehr weit verteilte Gesichter
// erzwingen ein Verschieben. In diesen Fällen gilt die Reihenfolge Bildgrenze
// vor Padding vor Zentrierung. Ein leeres Rechteck bedeutet "kein Ergebnis",
// der Aufrufer fällt dann auf den mittigen Standardausschnitt zurück.
func cropForFaces(bounds image.Rectangle, faces []image.Rectangle, area image.Rectangle) image.Rectangle {
	group := faceGroup(bounds, faces)
	if group.Empty() || bounds.Empty() || area.Empty() {
		return image.Rectangle{}
	}

	const (
		// padding ist der Anteil der Ausschnitthöhe, der links, rechts und
		// oben frei bleibt, damit keine Gesichter angeschnitten werden.
		padding = 0.10

		// groupBand ist die daraus folgende maximale Höhe der Gesichtsbox:
		// Ihre Mitte liegt auf 1/4 der Höhe, oben bleiben 10 % frei, also
		// 2*(0.25-0.10) = 0.30.
		groupBand = 0.30
	)

	aspect := float64(area.Dx()) / float64(area.Dy())
	if aspect <= 2*padding {
		// Ein derart schmaler Bereich lässt das seitliche Padding nicht zu.
		// Der mittige Standardausschnitt ist dann die ehrlichere Antwort.
		return image.Rectangle{}
	}

	// Es gilt cropW = cropH*aspect. Die schärfere der beiden Forderungen
	// bestimmt den Zoom, sodass die Gesichtsbox auf mindestens einer Achse
	// formatfüllend im vorgesehenen Bereich sitzt.
	cropH := max(
		float64(group.Dx())/(aspect-2*padding),
		float64(group.Dy())/groupBand,
	)

	// Nicht über die native Druckauflösung hinaus vergrößern. Ohne diese
	// Grenze würde eine einzelne Person nah an der Kamera auf einen winzigen
	// Ausschnitt zoomen, der auf 300 dpi sichtbar unscharf ausgedruckt wird.
	cropH = max(cropH, float64(area.Dy()))

	// Der Ausschnitt kann das Original nicht überschreiten; das Verhältnis
	// bleibt dabei erhalten. Das geht dem Padding bewusst vor.
	cropH = min(cropH, float64(bounds.Dy()))
	cropH = min(cropH, float64(bounds.Dx())/aspect)

	h := max(1, int(cropH+0.5))
	w := max(1, int(cropH*aspect+0.5))
	w, h = min(w, bounds.Dx()), min(h, bounds.Dy())

	// Horizontal zentriert, vertikal so, dass die Gesichtsmitte auf 1/4 der
	// Ausschnitthöhe liegt – also mittig in der oberen Hälfte.
	x := group.Min.X + group.Dx()/2 - w/2
	y := group.Min.Y + group.Dy()/2 - h/4

	// Ins Bild schieben statt beschneiden, damit das Seitenverhältnis hält.
	x = max(bounds.Min.X, min(x, bounds.Max.X-w))
	y = max(bounds.Min.Y, min(y, bounds.Max.Y-h))

	return image.Rect(x, y, x+w, y+h)
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
