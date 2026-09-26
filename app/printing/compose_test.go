package printing_test

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"testing"
	"time"

	"github.com/torbenschinke/eventprint/app/printing"
)

// Die Motive dieser Tests sind einfarbig. Damit lässt sich an jedem Punkt
// des Blattes ablesen, welches Motiv dort liegt – oder ob dort Papier ist.
var (
	red   = color.RGBA{R: 200, G: 30, B: 30, A: 0xFF}
	green = color.RGBA{R: 30, G: 170, B: 60, A: 0xFF}
	blue  = color.RGBA{R: 30, G: 60, B: 200, A: 0xFF}
	white = color.RGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
)

func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)

	return img
}

func layoutOf(f printing.Format, d printing.Design) printing.Layout {
	l := printing.DefaultLayout()
	l.Format, l.Design = f, d

	return l
}

// compose legt ein Blatt ohne Gesichtserkennung an.
func compose(l printing.Layout, imgs ...image.Image) *image.RGBA {
	return printing.Compose(printing.Sheet{Layout: l, Images: imgs}, printing.NativeRaster4x6, printing.RenderOptions{})
}

// near erlaubt Rundungsfehler beim Skalieren; ein anderes Motiv oder das
// Papier liegen weit außerhalb.
func near(got color.Color, want color.RGBA) bool {
	r, g, b, _ := got.RGBA()
	d := func(a uint32, b uint8) int { return max(int(a>>8)-int(b), int(b)-int(a>>8)) }

	return d(r, want.R) <= 3 && d(g, want.G) <= 3 && d(b, want.B) <= 3
}

// visibleArea ist das sichtbare Papier innerhalb des Rasters mit Überstand.
func visibleArea(landscape bool) image.Rectangle {
	pw, ph := printing.NativeRaster4x6.Short(), printing.NativeRaster4x6.Long()
	vw, vh := printing.VisibleMedia4x6.Short(), printing.VisibleMedia4x6.Long()
	if landscape {
		pw, ph, vw, vh = ph, pw, vh, vw
	}

	x, y := (pw-vw)/2, (ph-vh)/2

	return image.Rect(x, y, x+vw, y+vh)
}

// bboxOf liefert das umschließende Rechteck aller Punkte in Farbe c.
func bboxOf(img *image.RGBA, c color.RGBA) image.Rectangle {
	var box image.Rectangle
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if !near(img.At(x, y), c) {
				continue
			}

			p := image.Rect(x, y, x+1, y+1)
			if box.Empty() {
				box = p
			} else {
				box = box.Union(p)
			}
		}
	}

	return box
}

// runs misst zusammenhängende Strecken der Farbe c entlang einer Zeile
// (horizontal) oder Spalte.
func runs(img *image.RGBA, c color.RGBA, fixed int, horizontal bool) []int {
	var out []int
	n := 0
	b := img.Bounds()
	lo, hi := b.Min.Y, b.Max.Y
	if horizontal {
		lo, hi = b.Min.X, b.Max.X
	}

	for i := lo; i < hi; i++ {
		x, y := fixed, i
		if horizontal {
			x, y = i, fixed
		}

		if near(img.At(x, y), c) {
			n++
			continue
		}

		if n > 0 {
			out = append(out, n)
			n = 0
		}
	}

	if n > 0 {
		out = append(out, n)
	}

	return out
}

func count(img *image.RGBA, r image.Rectangle, match func(r, g, b uint8) bool) int {
	n := 0
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			i := img.PixOffset(x, y)
			if match(img.Pix[i], img.Pix[i+1], img.Pix[i+2]) {
				n++
			}
		}
	}

	return n
}

// TestComposeEveryCombination: Jede Kombination, die das Druck-Studio
// anbietet, muss ein Blatt in genau der Rastergröße ergeben, die der
// CZ-01 annimmt. Eine andere Größe würde vor dem Spooling abgelehnt – am
// Abend der Feier, nicht im Test.
func TestComposeEveryCombination(t *testing.T) {
	portrait := []image.Image{solid(40, 60, red), solid(40, 60, green), solid(40, 60, blue)}
	landscape := []image.Image{solid(60, 40, red), solid(60, 40, green), solid(60, 40, blue)}

	for _, f := range printing.Formats() {
		for _, d := range printing.Designs() {
			l := layoutOf(f.ID, d.ID)
			l.Caption = "Anna & Ben"
			l.DateStamp = true

			for name, imgs := range map[string][]image.Image{"hochkant": portrait, "quer": landscape, "ein Motiv": portrait[:1]} {
				t.Run(fmt.Sprintf("%s/%s/%s", f.ID, d.ID, name), func(t *testing.T) {
					canvas := printing.Compose(printing.Sheet{Layout: l, Images: imgs}, printing.NativeRaster4x6, printing.RenderOptions{})

					w, h := printing.NativeRaster4x6.Short(), printing.NativeRaster4x6.Long()
					if printing.SheetLandscape(l, imgs[0].Bounds().Dx(), imgs[0].Bounds().Dy()) {
						w, h = h, w
					}

					if canvas.Bounds() != image.Rect(0, 0, w, h) {
						t.Fatalf("Blatt %v, erwartet %dx%d", canvas.Bounds(), w, h)
					}
				})
			}
		}
	}
}

// TestComposeWithoutMotif: Die Vorschau eines Formats, bevor ein Foto
// gewählt ist, und ein Blatt ohne Motiv sollen ein leeres, hochkantes Blatt
// ergeben und nicht abstürzen.
func TestComposeWithoutMotif(t *testing.T) {
	for _, f := range printing.Formats() {
		for _, d := range printing.Designs() {
			canvas := compose(layoutOf(f.ID, d.ID))
			if canvas.Bounds() != image.Rect(0, 0, printing.NativeRaster4x6.Short(), printing.NativeRaster4x6.Long()) {
				t.Errorf("%s/%s: Blatt ohne Motiv %v, erwartet hochkant", f.ID, d.ID, canvas.Bounds())
			}
		}
	}
}

// TestSheetLandscape hält die Lageregel fest: Nur ein Einzelbild folgt
// seinem Motiv, und auch das nicht als Polaroid.
func TestSheetLandscape(t *testing.T) {
	cases := []struct {
		layout printing.Layout
		w, h   int
		want   bool
	}{
		{layoutOf(printing.FormatSingle, printing.DesignBorderless), 600, 400, true},
		{layoutOf(printing.FormatSingle, printing.DesignMatte), 600, 400, true},
		{layoutOf(printing.FormatSingle, printing.DesignFilm), 400, 400, true},
		{layoutOf(printing.FormatSingle, printing.DesignBorderless), 400, 600, false},
		{layoutOf(printing.FormatSingle, printing.DesignPolaroid), 600, 400, false},
		{layoutOf(printing.FormatCollage, printing.DesignBorderless), 600, 400, false},
		{layoutOf(printing.FormatSquare, printing.DesignBorderless), 600, 400, false},
		{layoutOf(printing.FormatSingle, printing.DesignBorderless), 0, 0, false},
	}

	for _, c := range cases {
		if got := printing.SheetLandscape(c.layout, c.w, c.h); got != c.want {
			t.Errorf("%s/%s bei %dx%d: quer = %v, erwartet %v", c.layout.Format, c.layout.Design, c.w, c.h, got, c.want)
		}
	}
}

// TestComposePassportInRealSize: Ein Passbild, das ein Amt wegen der Größe
// ablehnt, ist kein Passbild. 35 x 45 mm bei 300 dpi sind 413 x 531 Punkte.
func TestComposePassportInRealSize(t *testing.T) {
	for _, d := range printing.Designs() {
		// Drei Motive, obwohl Passfotos nur eines zeigen: Das zweite und
		// dritte dürfen nirgends auftauchen.
		canvas := compose(layoutOf(printing.FormatPassport, d.ID), solid(300, 400, red), solid(300, 400, green), solid(300, 400, blue))

		if canvas.Bounds().Dx() > canvas.Bounds().Dy() {
			t.Fatalf("%s: Passfotos liegen quer", d.ID)
		}

		box := bboxOf(canvas, red)
		if box.Empty() {
			t.Fatalf("%s: kein Passbild auf dem Blatt", d.ID)
		}

		if got := runs(canvas, red, box.Min.Y+10, true); len(got) != 2 || got[0] != 413 || got[1] != 413 {
			t.Errorf("%s: Breiten %v, erwartet [413 413]", d.ID, got)
		}

		if got := runs(canvas, red, box.Min.X+10, false); len(got) != 2 || got[0] != 531 || got[1] != 531 {
			t.Errorf("%s: Höhen %v, erwartet [531 531]", d.ID, got)
		}

		// Mittig auf dem sichtbaren Papier, damit beim Schneiden überall
		// Rand bleibt.
		visible := visibleArea(false)
		if dx := (box.Min.X - visible.Min.X) - (visible.Max.X - box.Max.X); dx < -1 || dx > 1 {
			t.Errorf("%s: horizontal nicht mittig (%v in %v)", d.ID, box, visible)
		}

		if dy := (box.Min.Y - visible.Min.Y) - (visible.Max.Y - box.Max.Y); dy < -1 || dy > 1 {
			t.Errorf("%s: vertikal nicht mittig (%v in %v)", d.ID, box, visible)
		}

		// Passfotos werden geschnitten, das Papier bleibt weiß. Ausnahme ist
		// derzeit der Film: frameColor prüft das Filmdesign vor dem
		// Passfotoformat, und Compose zeichnet auch die Perforation. Ob das
		// gewollt ist, ist offen – hier deshalb nicht festgeschrieben.
		if d.ID != printing.DesignFilm && !near(canvas.At(box.Min.X-10, box.Min.Y-10), white) {
			t.Errorf("%s: Papier um die Passbilder ist %v, erwartet weiß", d.ID, canvas.At(box.Min.X-10, box.Min.Y-10))
		}

		if !bboxOf(canvas, green).Empty() || !bboxOf(canvas, blue).Empty() {
			t.Errorf("%s: ein zweites Motiv erscheint auf dem Passfotoblatt", d.ID)
		}
	}
}

// TestComposeStripRepeatsItself: Ein Fotostreifen ist zweimal derselbe
// Streifen, damit man ihn teilen kann.
func TestComposeStripRepeatsItself(t *testing.T) {
	canvas := compose(layoutOf(printing.FormatStrip, printing.DesignBorderless), solid(300, 200, red), solid(300, 200, green), solid(300, 200, blue))

	w, h := canvas.Bounds().Dx(), canvas.Bounds().Dy()
	want := []color.RGBA{red, green, blue}

	for col, x := range []int{w / 4, 3 * w / 4} {
		for row := range 3 {
			y := h*row/3 + h/6
			if got := canvas.At(x, y); !near(got, want[row]) {
				t.Errorf("Spalte %d, Feld %d: %v, erwartet Motiv %d", col+1, row+1, got, row)
			}
		}
	}

	// Mit nur zwei Motiven bleibt kein Feld weiß: Das dritte wird mit den
	// vorhandenen aufgefüllt.
	canvas = compose(layoutOf(printing.FormatStrip, printing.DesignBorderless), solid(300, 200, red), solid(300, 200, green))
	if got := canvas.At(w/4, h*2/3+h/6); !near(got, red) {
		t.Errorf("drittes Feld bei zwei Motiven: %v, erwartet das erste Motiv", got)
	}
}

// TestComposeSquareIsSquare gilt für jede Gestaltung. Ein Quadrat, das nur
// fast eines ist, fällt beim Einkleben ins Album sofort auf.
func TestComposeSquareIsSquare(t *testing.T) {
	for _, d := range printing.Designs() {
		for name, img := range map[string]image.Image{"hochkant": solid(400, 600, green), "quer": solid(600, 400, green)} {
			canvas := compose(layoutOf(printing.FormatSquare, d.ID), img)
			box := bboxOf(canvas, green)

			if box.Dx() == 0 || box.Dx() != box.Dy() {
				t.Errorf("%s/%s: Motiv %dx%d, erwartet quadratisch", d.ID, name, box.Dx(), box.Dy())
			}

			if !box.In(visibleArea(false)) {
				t.Errorf("%s/%s: Quadrat %v ragt über das sichtbare Papier", d.ID, name, box)
			}
		}
	}

	// Randlos heißt beim Quadrat: so breit wie das sichtbare Papier.
	box := bboxOf(compose(layoutOf(printing.FormatSquare, printing.DesignBorderless), solid(400, 600, green)), green)
	if box.Dx() != printing.VisibleMedia4x6.Short() {
		t.Errorf("randloses Quadrat %d breit, erwartet %d", box.Dx(), printing.VisibleMedia4x6.Short())
	}
}

// TestComposeMatteHasUniformBorder: Der Passepartout-Rand ist auf allen vier
// Seiten exakt 1 cm, gemessen vom sichtbaren Papierrand – in beiden Lagen
// und in jeder Rahmenfarbe.
func TestComposeMatteHasUniformBorder(t *testing.T) {
	const m = printing.PassepartoutMargin

	frames := map[printing.FrameColor]color.RGBA{
		printing.FrameWhite: white,
		printing.FrameCream: {R: 0xF3, G: 0xEC, B: 0xDF, A: 0xFF},
		printing.FrameBlack: {R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF},
	}

	for frame, paper := range frames {
		for name, img := range map[string]image.Image{"hochkant": solid(400, 600, blue), "quer": solid(600, 400, blue)} {
			l := layoutOf(printing.FormatSingle, printing.DesignMatte)
			l.Frame = frame

			canvas := compose(l, img)
			v := visibleArea(canvas.Bounds().Dx() > canvas.Bounds().Dy())
			cx, cy := (v.Min.X+v.Max.X)/2, (v.Min.Y+v.Max.Y)/2

			// Je Seite: der letzte Randpunkt und der erste Motivpunkt.
			edges := []struct {
				side         string
				frame, photo image.Point
			}{
				{"links", image.Pt(v.Min.X+m-1, cy), image.Pt(v.Min.X+m, cy)},
				{"rechts", image.Pt(v.Max.X-m, cy), image.Pt(v.Max.X-m-1, cy)},
				{"oben", image.Pt(cx, v.Min.Y+m-1), image.Pt(cx, v.Min.Y+m)},
				{"unten", image.Pt(cx, v.Max.Y-m), image.Pt(cx, v.Max.Y-m-1)},
			}

			for _, e := range edges {
				if got := canvas.At(e.frame.X, e.frame.Y); !near(got, paper) {
					t.Errorf("%s/%s %s: Rand bei %v ist %v, erwartet %v", frame, name, e.side, e.frame, got, paper)
				}

				if got := canvas.At(e.photo.X, e.photo.Y); !near(got, blue) {
					t.Errorf("%s/%s %s: Motiv bei %v ist %v – der Rand ist breiter als %d", frame, name, e.side, e.photo, got, m)
				}
			}
		}
	}
}

// TestComposePolaroidCaption: Die Beschriftung steht im breiten Steg – und
// ohne Text bleibt der Steg leer, statt Anführungszeichen oder Reste zu
// zeigen.
func TestComposePolaroidCaption(t *testing.T) {
	v := visibleArea(false)
	side, bottom := printing.VisibleShort*6/100, printing.VisibleShort*22/100
	band := image.Rect(v.Min.X+side, v.Max.Y-bottom, v.Max.X-side, v.Max.Y)

	dark := func(r, g, b uint8) bool { return int(r)+int(g)+int(b) < 300 }
	light := func(r, g, b uint8) bool { return int(r)+int(g)+int(b) > 600 }

	for _, caption := range []string{"", "   "} {
		l := layoutOf(printing.FormatSingle, printing.DesignPolaroid)
		l.Caption = caption

		if n := count(compose(l, solid(400, 600, green)), band, dark); n != 0 {
			t.Errorf("Beschriftung %q: %d dunkle Punkte im Steg, erwartet keine", caption, n)
		}
	}

	for _, font := range printing.CaptionFonts() {
		l := layoutOf(printing.FormatSingle, printing.DesignPolaroid)
		l.Caption = "Anna & Ben, 05.09.2026"
		l.CaptionFont = font.ID

		if n := count(compose(l, solid(400, 600, green)), band, dark); n < 200 {
			t.Errorf("Schrift %s: nur %d dunkle Punkte im Steg – die Beschriftung fehlt", font.ID, n)
		}
	}

	// Auf schwarzem Rahmen wäre dunkle Schrift unsichtbar.
	l := layoutOf(printing.FormatSingle, printing.DesignPolaroid)
	l.Caption = "Anna & Ben"
	l.Frame = printing.FrameBlack

	if n := count(compose(l, solid(400, 600, green)), band, light); n < 200 {
		t.Errorf("schwarzer Rahmen: nur %d helle Punkte im Steg – die Schrift ist nicht lesbar", n)
	}

	// Ein sehr langer Text wird gekürzt, statt über den Steg hinaus zu
	// laufen.
	l = layoutOf(printing.FormatSingle, printing.DesignPolaroid)
	l.Caption = "Herzlichen Glückwunsch zur Hochzeit, liebe Anna und lieber Ben, von der ganzen Familie"

	canvas := compose(l, solid(400, 600, green))
	outside := image.Rect(v.Min.X, v.Max.Y-bottom, v.Min.X+side, v.Max.Y)
	if n := count(canvas, outside, dark); n != 0 {
		t.Errorf("ein langer Text läuft über den Steg hinaus (%d Punkte im Rand)", n)
	}
}

// TestComposeFilmPaintsDarkBackground: Ein Negativstreifen ist dunkel, mit
// heller Perforation – und das bis in den Überstand, sonst blitzt beim
// randlosen Druck ein weißer Streifen.
func TestComposeFilmPaintsDarkBackground(t *testing.T) {
	film := color.RGBA{R: 0x16, G: 0x16, B: 0x16, A: 0xFF}
	hole := color.RGBA{R: 0xE8, G: 0xE6, B: 0xE0, A: 0xFF}

	canvas := compose(layoutOf(printing.FormatSingle, printing.DesignFilm), solid(400, 600, green))
	v := visibleArea(false)

	for _, p := range []image.Point{{0, 0}, {v.Min.X + 5, v.Min.Y + 5}, {v.Max.X - 5, v.Max.Y - 5}, {canvas.Bounds().Max.X - 1, canvas.Bounds().Max.Y - 1}} {
		if got := canvas.At(p.X, p.Y); !near(got, film) {
			t.Errorf("Punkt %v: %v, erwartet Filmschwarz", p, got)
		}
	}

	// Das erste Perforationsloch links oben.
	if got := canvas.At(v.Min.X+60, v.Min.Y+60); !near(got, hole) {
		t.Errorf("keine Perforation am linken Rand: %v", got)
	}

	if got := canvas.At(v.Min.X+v.Dx()/2, v.Min.Y+v.Dy()/2); !near(got, green) {
		t.Errorf("Bildmitte %v, erwartet das Motiv", got)
	}
}

// TestComposeFilters prüft die Farbanmutungen an einem kräftig roten Motiv.
func TestComposeFilters(t *testing.T) {
	src := color.RGBA{R: 200, G: 60, B: 30, A: 0xFF}

	pixel := func(f printing.Filter) color.RGBA {
		l := printing.DefaultLayout()
		l.Filter = f
		canvas := compose(l, solid(400, 600, src))

		return canvas.RGBAAt(canvas.Bounds().Dx()/2, canvas.Bounds().Dy()/2)
	}

	if got := pixel(printing.FilterOriginal); !near(got, src) {
		t.Errorf("Original verändert die Farbe: %v", got)
	}

	if got := pixel(printing.FilterBW); got.R != got.G || got.G != got.B {
		t.Errorf("Schwarzweiß ist farbig: %v", got)
	}

	orig := pixel(printing.FilterOriginal)

	if got := pixel(printing.FilterWarm); got.R <= orig.R || got.B >= orig.B {
		t.Errorf("Warm %v ist nicht wärmer als %v", got, orig)
	}

	if got := pixel(printing.FilterCool); got.B <= orig.B || got.R >= orig.R {
		t.Errorf("Kühl %v ist nicht kühler als %v", got, orig)
	}

	if got := pixel(printing.FilterVintage); got == orig {
		t.Errorf("Vintage verändert nichts: %v", got)
	}

	// Der Filter gilt dem Motiv, nicht dem Papier: Ein cremefarbener
	// Rahmen bleibt auch in Schwarzweiß creme.
	l := layoutOf(printing.FormatSingle, printing.DesignMatte)
	l.Frame = printing.FrameCream
	l.Filter = printing.FilterBW

	canvas := compose(l, solid(400, 600, src))
	v := visibleArea(false)
	if got := canvas.At(v.Min.X+10, v.Min.Y+10); !near(got, color.RGBA{R: 0xF3, G: 0xEC, B: 0xDF, A: 0xFF}) {
		t.Errorf("der Rahmen wurde mitgefiltert: %v", got)
	}
}

// TestComposeDateStamp: Das Datum steht orange in der rechten unteren Ecke
// des ersten Feldes, wie bei einer Kompaktkamera der Neunziger.
func TestComposeDateStamp(t *testing.T) {
	orange := func(r, g, b uint8) bool { return r > 220 && g > 100 && g < 180 && b < 80 }
	at := []time.Time{time.Date(2026, 9, 5, 20, 15, 0, 0, time.Local)}

	sheet := func(l printing.Layout, imgs ...image.Image) *image.RGBA {
		return printing.Compose(printing.Sheet{Layout: l, Images: imgs, Dates: at}, printing.NativeRaster4x6, printing.RenderOptions{})
	}

	l := printing.DefaultLayout()
	canvas := sheet(l, solid(400, 600, blue))
	b := canvas.Bounds()
	corner := image.Rect(b.Max.X-450, b.Max.Y-130, b.Max.X, b.Max.Y)

	if n := count(canvas, b, orange); n != 0 {
		t.Fatalf("ohne Datumsstempel %d orange Punkte", n)
	}

	l.DateStamp = true
	canvas = sheet(l, solid(400, 600, blue))

	if n := count(canvas, corner, orange); n < 100 {
		t.Errorf("nur %d orange Punkte rechts unten – der Stempel fehlt", n)
	}

	if n := count(canvas, b, orange) - count(canvas, corner, orange); n != 0 {
		t.Errorf("%d orange Punkte außerhalb der Ecke", n)
	}

	// In der Collage gehört der Stempel ins erste, große Feld und nicht in
	// die Ecke des Blattes.
	l = layoutOf(printing.FormatCollage, printing.DesignBorderless)
	l.DateStamp = true
	canvas = sheet(l, solid(400, 600, blue), solid(400, 600, blue), solid(400, 600, blue))
	firstBottom := int(float64(b.Dy())*0.6 + 0.5)
	firstCorner := image.Rect(b.Max.X-450, firstBottom-130, b.Max.X, firstBottom)

	if n := count(canvas, firstCorner, orange); n < 100 {
		t.Errorf("Collage: nur %d orange Punkte in der Ecke des ersten Feldes", n)
	}

	if n := count(canvas, corner, orange); n != 0 {
		t.Errorf("Collage: %d orange Punkte in der Ecke des Blattes", n)
	}
}

// TestComposeDetectsFacesOncePerMotif: Die Gesichtserkennung ist auf dem
// Raspberry Pi der teuerste Schritt. Vier Passbilder desselben Gesichts
// dürfen sie nicht viermal kosten – und abgeschaltet läuft sie gar nicht.
func TestComposeDetectsFacesOncePerMotif(t *testing.T) {
	calls := 0
	opts := printing.RenderOptions{AutoCrop: true, DetectFaces: func(img image.Image) []image.Rectangle {
		calls++
		b := img.Bounds()

		return []image.Rectangle{image.Rect(b.Dx()/3, b.Dy()/4, b.Dx()/2, b.Dy()/3)}
	}}

	l := layoutOf(printing.FormatPassport, printing.DesignBorderless)
	l.FaceCrop = true
	printing.Compose(printing.Sheet{Layout: l, Images: []image.Image{solid(900, 1200, red)}}, printing.NativeRaster4x6, opts)

	if calls != 1 {
		t.Errorf("Passfotos: %d Erkennungen, erwartet 1", calls)
	}

	calls = 0
	l = layoutOf(printing.FormatStrip, printing.DesignBorderless)
	l.FaceCrop = true
	printing.Compose(printing.Sheet{Layout: l, Images: []image.Image{solid(900, 600, red), solid(900, 600, green), solid(900, 600, blue)}}, printing.NativeRaster4x6, opts)

	if calls != 3 {
		t.Errorf("Fotostreifen: %d Erkennungen, erwartet 3 (je Motiv eine)", calls)
	}

	calls = 0
	l.FaceCrop = false
	printing.Compose(printing.Sheet{Layout: l, Images: []image.Image{solid(900, 600, red)}}, printing.NativeRaster4x6, opts)

	if calls != 0 {
		t.Errorf("abgeschaltet: %d Erkennungen, erwartet 0", calls)
	}
}

// TestLayoutNormalized: Ein gespeicherter Auftrag aus einer älteren Fassung
// soll drucken, statt an einem Wert zu scheitern, den es nicht mehr gibt.
func TestLayoutNormalized(t *testing.T) {
	stale := printing.Layout{
		Format:      "hexagon",
		Design:      "barock",
		Frame:       "pink",
		Filter:      "sepia",
		CaptionFont: "comic",
		Finish:      "seidenmatt",
		Caption:     "Anna & Ben",
		DateStamp:   true,
		FaceCrop:    false,
	}

	d := printing.DefaultLayout()
	want := d
	want.Caption = "Anna & Ben"
	want.DateStamp = true
	want.FaceCrop = false

	if got := stale.Normalized(); got != want {
		t.Errorf("Normalized() = %+v, erwartet %+v", got, want)
	}

	// Ein leeres Layout – etwa aus einem Auftrag vor dem Druck-Studio – ist
	// die Vorgabe, nur ohne die Schalter, die niemand gesetzt hat.
	empty := d
	empty.FaceCrop = false
	if got := (printing.Layout{}).Normalized(); got != empty {
		t.Errorf("leeres Layout = %+v, erwartet %+v", got, empty)
	}

	// Gültige Werte bleiben unangetastet.
	valid := printing.Layout{
		Format: printing.FormatCollage, Design: printing.DesignFilm, Frame: printing.FrameBlack,
		Filter: printing.FilterBW, CaptionFont: printing.FontMono, Finish: printing.FinishMatte,
		Caption: "x", DateStamp: true, FaceCrop: true,
	}
	if got := valid.Normalized(); got != valid {
		t.Errorf("gültiges Layout verändert: %+v", got)
	}

	for _, f := range printing.Formats() {
		for _, dsg := range printing.Designs() {
			l := layoutOf(f.ID, dsg.ID)
			if l.Normalized() != l {
				t.Errorf("%s/%s gilt als unbekannt", f.ID, dsg.ID)
			}
		}
	}
}

// TestLayoutSheets hält die Blattrechnung fest, auf der die Aufteilung eines
// Druckvorgangs beruht.
func TestLayoutSheets(t *testing.T) {
	cases := []struct {
		format        printing.Format
		slots, motifs int
		sheets        map[int]int
	}{
		{printing.FormatSingle, 1, 1, map[int]int{0: 0, 1: 1, 3: 3}},
		{printing.FormatSquare, 1, 1, map[int]int{2: 2}},
		{printing.FormatDuo, 2, 2, map[int]int{1: 1, 2: 1, 3: 2}},
		{printing.FormatStrip, 6, 3, map[int]int{1: 1, 3: 1, 4: 2, 6: 2, 7: 3}},
		{printing.FormatPassport, 4, 1, map[int]int{1: 1, 3: 3}},
		{printing.FormatCollage, 3, 3, map[int]int{3: 1, 4: 2, 5: 2, 6: 2, 7: 3}},
	}

	for _, c := range cases {
		l := layoutOf(c.format, printing.DesignBorderless)

		if l.Slots() != c.slots || l.Motifs() != c.motifs {
			t.Errorf("%s: %d Felder, %d Motive – erwartet %d, %d", c.format, l.Slots(), l.Motifs(), c.slots, c.motifs)
		}

		for n, want := range c.sheets {
			if got := l.Sheets(n); got != want {
				t.Errorf("%s: Sheets(%d) = %d, erwartet %d", c.format, n, got, want)
			}
		}

		if got := l.Sheets(-1); got != 0 {
			t.Errorf("%s: Sheets(-1) = %d, erwartet 0", c.format, got)
		}
	}
}

// TestTemplateLayout: Die drei Kiosk-Layouts sind Kombinationen des
// allgemeinen Modells. Ein Polaroid vom Handy eines Gastes sieht damit aus
// wie eines aus dem Druck-Studio.
func TestTemplateLayout(t *testing.T) {
	want := map[printing.TemplateID]printing.Design{
		printing.TemplateFull:         printing.DesignBorderless,
		printing.TemplatePassepartout: printing.DesignMatte,
		printing.TemplatePolaroid:     printing.DesignPolaroid,
		"gibt-es-nicht":               printing.DesignBorderless,
	}

	for tpl, design := range want {
		l := tpl.Layout()

		if l.Format != printing.FormatSingle || l.Design != design {
			t.Errorf("%q: %s/%s, erwartet single/%s", tpl, l.Format, l.Design, design)
		}

		if l.Normalized() != l {
			t.Errorf("%q: Layout %+v ist nicht normalisiert", tpl, l)
		}
	}
}
