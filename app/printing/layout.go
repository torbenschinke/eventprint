package printing

// Layout beschreibt vollständig, wie ein Blatt aussieht.
//
// Alle Formate teilen sich dasselbe Papier: 10 x 15 cm, mehr kann der CZ-01
// nicht. Passfotos, Fotostreifen oder Collagen sind deshalb Aufteilungen
// dieses einen Blattes und keine anderen Papiere. Wer schneidet, bekommt das
// Format; der Drucker weiß davon nichts.
type Layout struct {
	Format Format `json:"format,omitempty"`
	Design Design `json:"design,omitempty"`

	// Frame ist die Farbe von Rand und Steg bei Passepartout, Polaroid und
	// Galerie.
	Frame FrameColor `json:"frame,omitempty"`

	// Mat ist die Breite des Passepartout-Randes. Leer ist 1 cm, der Rand
	// der Kiosk-Layouts, den Gäste kennen.
	Mat MatWidth `json:"mat,omitempty"`

	Filter Filter `json:"filter,omitempty"`

	// Caption steht im breiten Steg des Polaroids.
	Caption string `json:"caption,omitempty"`

	CaptionFont CaptionFont `json:"font,omitempty"`

	// DateStamp druckt das Aufnahmedatum wie bei einer alten Kompaktkamera
	// in die Ecke.
	DateStamp bool `json:"stamp,omitempty"`

	Finish Finish `json:"finish,omitempty"`

	// FaceCrop richtet den Ausschnitt an erkannten Gesichtern aus.
	FaceCrop bool `json:"faceCrop,omitempty"`
}

// Format ist die Aufteilung des Blattes.
type Format string

const (
	FormatSingle   Format = "single"
	FormatSquare   Format = "square"
	FormatDuo      Format = "duo"
	FormatStrip    Format = "strip"
	FormatPassport Format = "passport"
	FormatCollage  Format = "collage"
)

// Design ist die Gestaltung des Rahmens.
type Design string

const (
	DesignBorderless Design = "borderless"
	DesignMatte      Design = "matte"
	DesignPolaroid   Design = "polaroid"
	DesignFilm       Design = "film"
	DesignGallery    Design = "gallery"
)

// FrameColor ist die Farbe des Rahmens.
type FrameColor string

const (
	FrameWhite FrameColor = "white"
	FrameCream FrameColor = "cream"
	FrameBlack FrameColor = "black"
)

// MatWidth ist die Breite des Passepartout-Randes.
type MatWidth string

const (
	MatWide   MatWidth = "10mm"
	MatMedium MatWidth = "7.5mm"
	MatNarrow MatWidth = "5mm"
)

// Dots ist die Randbreite in Rasterpunkten. Ein unbekannter Wert ist 1 cm.
func (m MatWidth) Dots() int {
	switch m {
	case MatNarrow:
		return mmToDots(5)
	case MatMedium:
		return mmToDots(7.5)
	default:
		return PassepartoutMargin
	}
}

// Filter ist eine Farbanmutung.
type Filter string

const (
	FilterOriginal Filter = "original"
	FilterBW       Filter = "bw"
	FilterWarm     Filter = "warm"
	FilterCool     Filter = "cool"
	FilterVintage  Filter = "vintage"
)

// CaptionFont ist die Schrift der Beschriftung.
type CaptionFont string

const (
	FontClassic CaptionFont = "classic"
	FontBold    CaptionFont = "bold"
	FontMono    CaptionFont = "mono"
)

// Finish ist die Oberfläche, die der CZ-01 beim Druck aufträgt.
type Finish string

const (
	FinishGlossy Finish = "glossy"
	FinishMatte  Finish = "matte"
)

// Choice ist ein Eintrag einer Auswahlliste für die Oberfläche.
type Choice[T ~string] struct {
	ID   T
	Name string
	Hint string
}

// Formats liefert die Formate in Anzeigereihenfolge.
func Formats() []Choice[Format] {
	return []Choice[Format]{
		{FormatSingle, "Einzelbild", "10 × 15 cm"},
		{FormatSquare, "Quadrat", "10 × 10 cm"},
		{FormatDuo, "Zwei Bilder", "2 × 10 × 7,5 cm"},
		{FormatStrip, "Fotostreifen", "2 × 5 × 15 cm"},
		{FormatPassport, "Passfotos", "4 × 3,5 × 4,5 cm"},
		{FormatCollage, "Collage", "3 Motive"},
	}
}

// Designs liefert die Rahmen in Anzeigereihenfolge.
func Designs() []Choice[Design] {
	return []Choice[Design]{
		{DesignBorderless, "Randlos", "bis an die Kante"},
		{DesignMatte, "Passepartout", "gleichmäßiger Rand"},
		{DesignPolaroid, "Polaroid", "breiter Steg unten"},
		{DesignFilm, "Film", "Negativstreifen"},
		{DesignGallery, "Galerie", "breiter Rand mit Linie"},
	}
}

// Frames liefert die Rahmenfarben.
func Frames() []Choice[FrameColor] {
	return []Choice[FrameColor]{
		{FrameWhite, "Weiß", "#FFFFFF"},
		{FrameCream, "Creme", "#F3ECDF"},
		{FrameBlack, "Schwarz", "#1A1A1A"},
	}
}

// Is meldet, ob m die Breite w meint; leer ist 1 cm.
func (m MatWidth) Is(w MatWidth) bool {
	if m == "" {
		m = MatWide
	}

	return m == w
}

// Mats liefert die Randbreiten des Passepartouts. Ein Zentimeter wirkt auf
// 10 × 15 cm wie ein Galerierahmen; für ein zurückhaltendes Blatt ist ein
// halber eleganter.
func Mats() []Choice[MatWidth] {
	return []Choice[MatWidth]{
		{MatNarrow, "5 mm", "schmal"},
		{MatMedium, "7,5 mm", "mittel"},
		{MatWide, "1 cm", "breit"},
	}
}

// Filters liefert die Farbanmutungen.
func Filters() []Choice[Filter] {
	return []Choice[Filter]{
		{FilterOriginal, "Original", ""},
		{FilterBW, "Schwarzweiß", ""},
		{FilterWarm, "Warm", ""},
		{FilterCool, "Kühl", ""},
		{FilterVintage, "Vintage", ""},
	}
}

// CaptionFonts liefert die Schriften der Beschriftung.
func CaptionFonts() []Choice[CaptionFont] {
	return []Choice[CaptionFont]{
		{FontClassic, "Klassisch", ""},
		{FontBold, "Kräftig", ""},
		{FontMono, "Schreibmaschine", ""},
	}
}

// DefaultLayout ist das Blatt, mit dem das Druck-Studio beginnt.
func DefaultLayout() Layout {
	return Layout{
		Format:      FormatSingle,
		Design:      DesignBorderless,
		Frame:       FrameWhite,
		Filter:      FilterOriginal,
		CaptionFont: FontClassic,
		Finish:      FinishGlossy,
		FaceCrop:    true,
	}
}

// Normalized ersetzt unbekannte oder leere Werte durch die Vorgabe. Ein
// gespeicherter Auftrag aus einer älteren Fassung soll drucken, statt an
// einem Wert zu scheitern, den es nicht mehr gibt.
func (l Layout) Normalized() Layout {
	d := DefaultLayout()
	if !known(Formats(), l.Format) {
		l.Format = d.Format
	}

	if !known(Designs(), l.Design) {
		l.Design = d.Design
	}

	if !known(Frames(), l.Frame) {
		l.Frame = d.Frame
	}

	if l.Mat != "" && !known(Mats(), l.Mat) {
		l.Mat = ""
	}

	if !known(Filters(), l.Filter) {
		l.Filter = d.Filter
	}

	if !known(CaptionFonts(), l.CaptionFont) {
		l.CaptionFont = d.CaptionFont
	}

	if l.Finish != FinishMatte {
		l.Finish = FinishGlossy
	}

	return l
}

// Slots ist die Zahl der Bildfelder eines Blattes.
func (l Layout) Slots() int {
	return len(cellsOf(l.Format))
}

// Motifs ist die Zahl verschiedener Fotos, die ein Blatt aufnimmt.
//
// Passfotos zeigen viermal dasselbe Gesicht, ein Fotostreifen ist zweimal
// derselbe Streifen, damit man ihn teilen kann.
func (l Layout) Motifs() int {
	switch l.Format {
	case FormatPassport:
		return 1
	case FormatStrip:
		return 3
	default:
		return l.Slots()
	}
}

// Sheets ist die Zahl der Blätter für n Fotos.
func (l Layout) Sheets(n int) int {
	m := l.Motifs()
	if n <= 0 || m <= 0 {
		return 0
	}

	return (n + m - 1) / m
}

// Layout ist das Blatt, das ein Kiosk-Layout beschreibt.
//
// Die drei Kiosk-Layouts sind Kombinationen des allgemeinen Modells; es gibt
// nur einen Renderer. So sieht ein Polaroid vom Handy eines Gastes genauso
// aus wie eines aus dem Druck-Studio.
func (t TemplateID) Layout() Layout {
	l := DefaultLayout()

	// Im Kiosk richtet sich nur das Polaroid nach Gesichtern, wie es immer
	// war: Ein formatfüllender Druck soll nicht plötzlich anders
	// beschnitten sein als der Gast es von der Vorschau kennt.
	l.FaceCrop = t == TemplatePolaroid

	switch t {
	case TemplatePassepartout:
		l.Design = DesignMatte
	case TemplatePolaroid:
		l.Design = DesignPolaroid
	}

	return l
}

func known[T ~string](choices []Choice[T], v T) bool {
	for _, c := range choices {
		if c.ID == v {
			return true
		}
	}

	return false
}
