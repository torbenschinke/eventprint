package printing

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Sheet ist ein Blatt mit seinen Motiven.
type Sheet struct {
	Layout Layout

	// Images sind die Motive in der Reihenfolge der Auswahl, bereits
	// aufgerichtet.
	Images []image.Image

	// Dates sind die Aufnahmezeitpunkte, parallel zu Images. Sie werden nur
	// für den Datumsstempel gebraucht.
	Dates []time.Time
}

// cell ist ein Bildfeld in Prozent des Inhaltsbereichs.
type cell struct{ l, t, w, h float64 }

// cellsOf liefert die Aufteilung eines Formats.
func cellsOf(f Format) []cell {
	switch f {
	case FormatDuo:
		return []cell{{0, 0, 100, 50}, {0, 50, 100, 50}}
	case FormatStrip:
		third := 100.0 / 3
		return []cell{
			{0, 0, 50, third}, {0, third, 50, third}, {0, 2 * third, 50, third},
			{50, 0, 50, third}, {50, third, 50, third}, {50, 2 * third, 50, third},
		}
	case FormatPassport:
		return []cell{{0, 0, 50, 50}, {50, 0, 50, 50}, {0, 50, 50, 50}, {50, 50, 50, 50}}
	case FormatCollage:
		return []cell{{0, 0, 100, 60}, {0, 60, 50, 40}, {50, 60, 50, 40}}
	default:
		return []cell{{0, 0, 100, 100}}
	}
}

// Maße der Gestaltungen in Rasterpunkten bei 300 dpi, bezogen auf das
// Hochformat. Der Passepartout-Rand und die Polaroid-Proportionen sind die
// der bisherigen Kiosk-Layouts und dürfen sich nicht ändern: Gäste kennen
// diese Blätter.
const (
	polaroidSide   = VisibleShort * 6 / 100
	polaroidBottom = VisibleShort * 22 / 100
	filmEdge       = 120
	filmEnd        = 48
	galleryMargin  = 140
	galleryLine    = 4
	galleryOffset  = 22

	// VisibleShort ist die kurze Kante der sichtbaren Fläche.
	VisibleShort = 1200
)

// padding liefert den Rand um den Inhaltsbereich und den Abstand zwischen
// den Feldern, in Rasterpunkten und bezogen auf die tatsächliche Lage.
func padding(d Design, landscape bool) (left, top, right, bottom, gap int) {
	switch d {
	case DesignMatte:
		return PassepartoutMargin, PassepartoutMargin, PassepartoutMargin, PassepartoutMargin, PassepartoutMargin / 2
	case DesignPolaroid:
		return polaroidSide, polaroidSide, polaroidSide, polaroidBottom, polaroidSide / 2
	case DesignFilm:
		if landscape {
			return filmEnd, filmEdge, filmEnd, filmEdge, 24
		}

		return filmEdge, filmEnd, filmEdge, filmEnd, 24
	case DesignGallery:
		return galleryMargin, galleryMargin, galleryMargin, galleryMargin, galleryOffset * 3
	default:
		return 0, 0, 0, 0, 0
	}
}

// SheetLandscape meldet, ob ein Blatt quer liegt.
//
// Nur ein Einzelbild folgt der Lage seines Motivs; alle Aufteilungen sind
// für das Hochformat entworfen. Ein Polaroid bleibt hochkant, sonst säße der
// breite Steg an der falschen Kante.
func SheetLandscape(l Layout, imgW, imgH int) bool {
	if l.Format != FormatSingle || l.Design == DesignPolaroid {
		return false
	}

	return imgW > 0 && imgH > 0 && imgW >= imgH
}

// frameColor liefert die Papierfarbe eines Blattes.
func frameColor(l Layout) color.RGBA {
	if l.Format == FormatPassport {
		return color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}

	if l.Design == DesignFilm {
		return color.RGBA{R: 0x16, G: 0x16, B: 0x16, A: 0xFF}
	}

	if l.Design == DesignBorderless || l.Format == FormatPassport {
		return color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}

	switch l.Frame {
	case FrameCream:
		return color.RGBA{R: 0xF3, G: 0xEC, B: 0xDF, A: 0xFF}
	case FrameBlack:
		return color.RGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
	default:
		return color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	}
}

// Compose legt ein Blatt in voller Druckauflösung an, noch ohne
// Druckerkalibrierung.
func Compose(s Sheet, raster Raster, opts RenderOptions) *image.RGBA {
	l := s.Layout.Normalized()
	if len(s.Images) == 0 {
		s.Images = []image.Image{placeholderImage()}
	}

	first := s.Images[0].Bounds()
	landscape := SheetLandscape(l, first.Dx(), first.Dy())

	pageW, pageH := raster.Short(), raster.Long()
	visW, visH := VisibleMedia4x6.Short(), VisibleMedia4x6.Long()
	if landscape {
		pageW, pageH = pageH, pageW
		visW, visH = visH, visW
	}

	canvas := image.NewRGBA(image.Rect(0, 0, pageW, pageH))
	paper := frameColor(l)
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(paper), image.Point{}, draw.Src)

	visible := image.Rect((pageW-visW)/2, (pageH-visH)/2, (pageW-visW)/2+visW, (pageH-visH)/2+visH)

	areas := cellAreas(l, canvas.Bounds(), visible, landscape)
	// Die Einstellung des Geräts schaltet die Erkennung ab, das Blatt kann
	// sie zusätzlich abwählen. Beides muss zustimmen.
	faces := faceCache{detect: opts.DetectFaces, enabled: l.FaceCrop && opts.AutoCrop}

	for i, area := range areas {
		src := s.Images[motifIndex(l, i, len(s.Images))]
		if crop := faces.crop(src, area); !crop.Empty() {
			drawCoverCrop(canvas, area, src, crop)
		} else {
			drawCover(canvas, area, src)
		}

		applyFilter(canvas, area, l.Filter)

		if l.Design == DesignGallery {
			strokeRect(canvas, area.Inset(-galleryOffset), galleryLine, color.RGBA{R: 0x9A, G: 0x9A, B: 0x9A, A: 0xFF})
		}

		if l.Format == FormatPassport {
			strokeRect(canvas, area.Inset(-2), 2, color.RGBA{R: 0xD0, G: 0xD0, B: 0xD0, A: 0xFF})
		}
	}

	if l.Design == DesignFilm && l.Format != FormatPassport {
		drawSprockets(canvas, visible, landscape)
	}

	if l.Design == DesignPolaroid && l.Format != FormatPassport && strings.TrimSpace(l.Caption) != "" {
		band := image.Rect(visible.Min.X+polaroidSide, visible.Max.Y-polaroidBottom, visible.Max.X-polaroidSide, visible.Max.Y)
		drawCaption(canvas, band, l.Caption, l.CaptionFont, contrastInk(paper))
	}

	if l.DateStamp && len(areas) > 0 {
		var at time.Time
		if len(s.Dates) > 0 {
			at = s.Dates[motifIndex(l, 0, len(s.Dates))]
		}

		if at.IsZero() {
			at = time.Now()
		}

		drawDateStamp(canvas, areas[0], at)
	}

	return canvas
}

// motifIndex ordnet einem Feld sein Motiv zu.
func motifIndex(l Layout, cellIdx, n int) int {
	if n <= 0 {
		return 0
	}

	switch l.Format {
	case FormatPassport:
		return 0
	case FormatStrip:
		return (cellIdx % 3) % n
	default:
		return cellIdx % n
	}
}

// cellAreas rechnet die Felder eines Blattes in Rasterpunkte um.
func cellAreas(l Layout, page, visible image.Rectangle, landscape bool) []image.Rectangle {
	if l.Format == FormatPassport {
		return passportAreas(visible)
	}

	left, top, right, bottom, gap := padding(l.Design, landscape)

	// Randlos reicht bis in den Überstand, damit auch bei leichtem Versatz
	// im Drucker kein weißer Streifen bleibt.
	content := page
	if l.Design != DesignBorderless {
		content = image.Rect(visible.Min.X+left, visible.Min.Y+top, visible.Max.X-right, visible.Max.Y-bottom)
	}

	if l.Format == FormatSquare {
		side := min(content.Dx(), content.Dy())
		if l.Design == DesignBorderless {
			side = min(visible.Dx(), visible.Dy())
		}

		x := content.Min.X + (content.Dx()-side)/2
		y := content.Min.Y + (content.Dy()-side)/2

		return []image.Rectangle{image.Rect(x, y, x+side, y+side)}
	}

	cells := cellsOf(l.Format)
	out := make([]image.Rectangle, 0, len(cells))
	for _, c := range cells {
		r := image.Rect(
			content.Min.X+int(float64(content.Dx())*c.l/100+0.5),
			content.Min.Y+int(float64(content.Dy())*c.t/100+0.5),
			content.Min.X+int(float64(content.Dx())*(c.l+c.w)/100+0.5),
			content.Min.Y+int(float64(content.Dy())*(c.t+c.h)/100+0.5),
		)

		// Der Abstand gilt nur zwischen Feldern, nicht am Rand: Dort sorgt
		// schon die Gestaltung für Luft.
		half := gap / 2
		if c.l > 0 {
			r.Min.X += half
		}

		if c.t > 0 {
			r.Min.Y += half
		}

		if c.l+c.w < 99.9 {
			r.Max.X -= half
		}

		if c.t+c.h < 99.9 {
			r.Max.Y -= half
		}

		out = append(out, r)
	}

	return out
}

// passportAreas setzt vier Passbilder in echter Größe von 35 x 45 mm.
//
// Hier zählt der Millimeter: Ein Passbild, das ein Amt wegen der Größe
// ablehnt, ist kein Passbild.
func passportAreas(visible image.Rectangle) []image.Rectangle {
	w := mmToDots(35)
	h := mmToDots(45)
	gap := mmToDots(5)

	totalW := 2*w + gap
	totalH := 2*h + gap
	x0 := visible.Min.X + (visible.Dx()-totalW)/2
	y0 := visible.Min.Y + (visible.Dy()-totalH)/2

	return []image.Rectangle{
		image.Rect(x0, y0, x0+w, y0+h),
		image.Rect(x0+w+gap, y0, x0+2*w+gap, y0+h),
		image.Rect(x0, y0+h+gap, x0+w, y0+2*h+gap),
		image.Rect(x0+w+gap, y0+h+gap, x0+2*w+gap, y0+2*h+gap),
	}
}

func mmToDots(mm float64) int {
	return int(mm/25.4*DPI + 0.5)
}

// faceCache erkennt Gesichter je Motiv nur einmal, auch wenn es mehrere
// Felder füllt – die Erkennung ist auf einem Raspberry Pi der teuerste
// Schritt des ganzen Renderns.
type faceCache struct {
	detect  FaceDetector
	enabled bool
	known   map[image.Image][]image.Rectangle
}

func (f *faceCache) crop(src image.Image, area image.Rectangle) image.Rectangle {
	if !f.enabled || f.detect == nil {
		return image.Rectangle{}
	}

	if f.known == nil {
		f.known = map[image.Image][]image.Rectangle{}
	}

	faces, ok := f.known[src]
	if !ok {
		faces = f.detect(src)
		f.known[src] = faces
	}

	return cropForFaces(src.Bounds(), faces, area)
}

// applyFilter verändert die Farben eines Feldes.
func applyFilter(img *image.RGBA, area image.Rectangle, f Filter) {
	if f == FilterOriginal || f == "" {
		return
	}

	area = area.Intersect(img.Bounds())
	for y := area.Min.Y; y < area.Max.Y; y++ {
		i := img.PixOffset(area.Min.X, y)
		for x := area.Min.X; x < area.Max.X; x++ {
			r, g, b := float64(img.Pix[i]), float64(img.Pix[i+1]), float64(img.Pix[i+2])
			switch f {
			case FilterBW:
				v := 0.299*r + 0.587*g + 0.114*b
				v = (v-128)*1.08 + 128
				r, g, b = v, v, v
			case FilterWarm:
				r, g, b = r*1.06+6, g*1.01+2, b*0.9
			case FilterCool:
				r, g, b = r*0.92, g*0.99, b*1.06+8
			case FilterVintage:
				sr := 0.393*r + 0.769*g + 0.189*b
				sg := 0.349*r + 0.686*g + 0.168*b
				sb := 0.272*r + 0.534*g + 0.131*b
				r = 0.55*r + 0.45*sr
				g = 0.55*g + 0.45*sg
				b = 0.55*b + 0.45*sb
				r, g, b = r*0.88+22, g*0.88+18, b*0.88+14
			}

			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = clamp8(r), clamp8(g), clamp8(b)
			i += 4
		}
	}
}

func clamp8(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	default:
		return uint8(v + 0.5)
	}
}

// strokeRect zeichnet einen Rahmen der Breite w innen entlang r.
func strokeRect(img *image.RGBA, r image.Rectangle, w int, c color.RGBA) {
	u := image.NewUniform(c)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+w), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-w, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+w, r.Max.Y), u, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-w, r.Min.Y, r.Max.X, r.Max.Y), u, image.Point{}, draw.Src)
}

// drawSprockets zeichnet die Perforation eines Negativstreifens.
func drawSprockets(img *image.RGBA, visible image.Rectangle, landscape bool) {
	hole := image.NewUniform(color.RGBA{R: 0xE8, G: 0xE6, B: 0xE0, A: 0xFF})
	const holeLong, holeShort, pitch = 44, 30, 76

	if landscape {
		for x := visible.Min.X + pitch/2; x+holeLong < visible.Max.X; x += pitch {
			for _, y := range []int{visible.Min.Y + (filmEdge-holeShort)/2, visible.Max.Y - (filmEdge+holeShort)/2} {
				draw.Draw(img, image.Rect(x, y, x+holeLong, y+holeShort), hole, image.Point{}, draw.Src)
			}
		}

		return
	}

	for y := visible.Min.Y + pitch/2; y+holeLong < visible.Max.Y; y += pitch {
		for _, x := range []int{visible.Min.X + (filmEdge-holeShort)/2, visible.Max.X - (filmEdge+holeShort)/2} {
			draw.Draw(img, image.Rect(x, y, x+holeShort, y+holeLong), hole, image.Point{}, draw.Src)
		}
	}
}

// contrastInk liefert eine lesbare Schriftfarbe auf dem Papier.
func contrastInk(paper color.RGBA) color.RGBA {
	if int(paper.R)+int(paper.G)+int(paper.B) < 3*128 {
		return color.RGBA{R: 0xF2, G: 0xF2, B: 0xF2, A: 0xFF}
	}

	return color.RGBA{R: 0x2A, G: 0x2A, B: 0x2A, A: 0xFF}
}

var (
	fontsOnce sync.Once
	fonts     map[CaptionFont]*opentype.Font
	stampFont *opentype.Font
)

// loadFonts parst die eingebetteten Go-Schriften einmal. Sie liegen im
// Modul golang.org/x/image und brauchen keine Datei auf dem Gerät.
func loadFonts() {
	fontsOnce.Do(func() {
		parse := func(b []byte) *opentype.Font {
			f, err := opentype.Parse(b)
			if err != nil {
				panic(fmt.Sprintf("embedded font broken: %v", err))
			}

			return f
		}

		fonts = map[CaptionFont]*opentype.Font{
			FontClassic: parse(gomedium.TTF),
			FontBold:    parse(gobold.TTF),
			FontMono:    parse(gomonobold.TTF),
		}
		stampFont = parse(gomonobold.TTF)
	})
}

// drawCaption setzt die Beschriftung mittig in den Steg. Zu lange Texte
// werden kleiner gesetzt statt abgeschnitten; erst unterhalb einer lesbaren
// Größe wird gekürzt.
func drawCaption(img *image.RGBA, band image.Rectangle, text string, f CaptionFont, ink color.RGBA) {
	loadFonts()
	otf := fonts[f]
	if otf == nil {
		otf = fonts[FontClassic]
	}

	text = strings.TrimSpace(text)
	maxW := band.Dx() - 40

	for size := 84.0; size >= 40; size -= 4 {
		face, err := opentype.NewFace(otf, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return
		}

		width := font.MeasureString(face, text).Ceil()
		if width <= maxW || size <= 40 {
			for width > maxW && len([]rune(text)) > 1 {
				r := []rune(text)
				text = string(r[:len(r)-2]) + "…"
				width = font.MeasureString(face, text).Ceil()
			}

			m := face.Metrics()
			y := band.Min.Y + (band.Dy()+m.Ascent.Ceil()-m.Descent.Ceil())/2
			d := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: face,
				Dot: fixed.P(band.Min.X+(band.Dx()-width)/2, y)}
			d.DrawString(text)
			_ = face.Close()

			return
		}

		_ = face.Close()
	}
}

// drawDateStamp druckt das Datum orange in die rechte untere Ecke, wie es
// Kompaktkameras der Neunziger taten.
func drawDateStamp(img *image.RGBA, area image.Rectangle, at time.Time) {
	loadFonts()
	face, err := opentype.NewFace(stampFont, &opentype.FaceOptions{Size: 48, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return
	}
	defer func() { _ = face.Close() }()

	text := at.Local().Format("02 1 '06")
	width := font.MeasureString(face, text).Ceil()
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.RGBA{R: 0xFF, G: 0x8A, B: 0x1F, A: 0xFF}), Face: face,
		Dot: fixed.P(area.Max.X-width-36, area.Max.Y-36)}
	d.DrawString(text)
}

// RenderSheet erzeugt das druckfertige JPEG eines Blattes.
func RenderSheet(s Sheet, raster Raster, opts RenderOptions) ([]byte, error) {
	if raster != NativeRaster4x6 {
		return nil, fmt.Errorf("unsupported print raster %dx%d: expected %dx%d",
			raster.Width, raster.Height, NativeRaster4x6.Width, NativeRaster4x6.Height)
	}

	canvas := Compose(s, raster, opts)
	applyCalibration(canvas)
	compensateDriverRotation(canvas)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("cannot encode print image: %w", err)
	}

	return withJFIFHeader(buf.Bytes()), nil
}

// PreviewSheet erzeugt eine verkleinerte Vorschau der sichtbaren Fläche.
//
// Sie entsteht mit demselben Renderer wie der Ausdruck, nur ohne
// Druckerkalibrierung und Drehausgleich. Was die Vorschau zeigt, kommt aus
// dem Drucker – eine zweite, nachgebaute Darstellung könnte davon abweichen.
func PreviewSheet(s Sheet, maxEdge int, opts RenderOptions) ([]byte, error) {
	canvas := Compose(s, NativeRaster4x6, opts)

	b := canvas.Bounds()
	visW, visH := VisibleMedia4x6.Short(), VisibleMedia4x6.Long()
	if b.Dx() > b.Dy() {
		visW, visH = visH, visW
	}

	visible := image.Rect((b.Dx()-visW)/2, (b.Dy()-visH)/2, (b.Dx()-visW)/2+visW, (b.Dy()-visH)/2+visH)

	if maxEdge <= 0 {
		maxEdge = 900
	}

	scale := float64(maxEdge) / float64(max(visW, visH))
	dst := image.NewRGBA(image.Rect(0, 0, max(1, int(float64(visW)*scale)), max(1, int(float64(visH)*scale))))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), canvas, visible, draw.Src, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 88}); err != nil {
		return nil, fmt.Errorf("cannot encode preview: %w", err)
	}

	return buf.Bytes(), nil
}
