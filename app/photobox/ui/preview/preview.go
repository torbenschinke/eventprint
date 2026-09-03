// Package preview renders the print-template selector shared by photobox and photoupld.
package preview

import (
	"go.wdy.de/nago/application/image"
	httpimage "go.wdy.de/nago/application/image/http"
	icons "go.wdy.de/nago/presentation/icons/hero/solid"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"

	"github.com/torbenschinke/eventprint/app/printing"
)

const (
	paperWhite ui.Color = "#FFFFFF"
	// Die kurze und die lange Kante des Blattes im Verhältnis 2:3.
	paperShort = ui.L96
	paperLong  = ui.Length("9rem")
	// 1 cm Rand auf einem 10,16 cm breiten Blatt sind 9,84 % der kurzen Kante.
	// Bezogen auf paperShort (6rem) also 0,59rem – auf allen vier Seiten
	// gleich, wie beim Druck.
	passepartoutInset = ui.Length("0.59rem")
	polaroidSide      = ui.Length("0.55rem")
	polaroidFoot      = ui.Length("2rem")
	previewPixels     = 256
)

// Selector displays all templates using a prepared Nago image pyramid.
//
// imgW und imgH sind die Maße des Motivs. Sie entscheiden, ob das Blatt quer
// oder hochkant gezeichnet wird – ohne sie zeigte die Vorschau bei einem
// Querformatfoto ein hochkantes Blatt und damit einen anderen Ausschnitt als
// der Ausdruck. Sind sie unbekannt (0), bleibt es beim Hochformat.
func Selector(img image.ID, imgW, imgH int, selected *core.State[printing.TemplateID]) core.View {
	var cards []core.View
	for _, tpl := range printing.Templates() {
		cards = append(cards, card(img, imgW, imgH, tpl, selected))
	}

	return ui.HStack(cards...).Gap(ui.L16).Alignment(ui.Stretch).Wrap(true)
}

func card(img image.ID, imgW, imgH int, tpl printing.Template, selected *core.State[printing.TemplateID]) core.View {
	active := selected.Get() == tpl.ID

	// Die Auswahl wird bewusst über mehrere Kanäle gleichzeitig angezeigt.
	//
	// Ein reiner Farbwechsel am Rahmen genügte nicht: Auf dem Smartphone ist
	// die Linie zu fein, im Darkmode überstrahlt das weiße Papier daneben
	// jede Randfarbe, und bei Farbfehlsichtigkeit fällt sie ganz aus. Fläche,
	// Helligkeit und ein Symbol wirken dagegen unabhängig von Theme und
	// Sehvermögen.
	borderColor := ui.M5
	borderWidth := ui.L1
	background := ui.Color("")
	labelColor := ui.M8
	opacity := 0.55

	if active {
		borderColor = ui.I0
		borderWidth = ui.L2
		background = ui.M2
		labelColor = ui.I0
		opacity = 1
	}

	label := ui.HStack(
		ui.If(active, ui.ImageIcon(icons.CheckCircle).FillColor(ui.I0).Frame(ui.Frame{}.Size(ui.L20, ui.L20))),
		ui.Text(tpl.Name).Font(ui.LabelLarge).Color(labelColor),
	).Gap(ui.L4).Alignment(ui.Center)

	// Der Zustand gehört in die Beschriftung, sonst liest ein Screenreader
	// drei gleich klingende Schaltflächen vor.
	accessibility := tpl.Name + ". " + tpl.Description
	if active {
		accessibility = tpl.Name + ", ausgewählt. " + tpl.Description
	}

	return ui.VStack(
		Paper(img, imgW, imgH, tpl.ID),
		label,
		ui.Text(tpl.Description).Font(ui.BodySmall).TextAlignment(ui.TextAlignCenter),
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		Action(func() { selected.Set(tpl.ID) }).
		BackgroundColor(background).
		Opacity(opacity).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12).Width(borderWidth).Color(borderColor)).
		Frame(ui.Frame{Width: ui.L200}).
		AccessibilityLabel(accessibility)
}

// Paper renders the selected image in the proportions of a 10x15 cm print.
//
// Das Blatt wird genauso gedreht wie beim Druck. Maßgeblich ist dafür
// ausschließlich [printing.PaperLandscape]; eine eigene Regel an dieser Stelle
// wäre eine zweite Wahrheit, die bei der nächsten Änderung abdriftet.
func Paper(img image.ID, imgW, imgH int, tpl printing.TemplateID) core.View {
	width, height := paperShort, paperLong
	if printing.PaperLandscape(tpl, imgW, imgH) {
		width, height = paperLong, paperShort
	}

	return ui.VStack(motif(img, tpl)).
		Alignment(ui.Center).
		BackgroundColor(paperWhite).
		Border(ui.Border{}.Radius(ui.L4).Width(ui.L1).Color(ui.M5)).
		Frame(ui.Frame{Width: width, Height: height})
}

func motif(img image.ID, tpl printing.TemplateID) core.View {
	if img == "" {
		return nil
	}

	// FitCover selects a suitable precomputed pyramid element instead of the original.
	uri := httpimage.URI(img, image.FitCover, previewPixels, previewPixels)
	switch tpl {
	case printing.TemplatePassepartout:
		// FitCover, nicht FitContain: Der Rahmen hat Vorrang, das Motiv wird
		// dafür beschnitten. Genau das muss die Vorschau zeigen.
		return ui.Image().URI(uri).ObjectFit(ui.FitCover).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).Padding(ui.Padding{}.All(passepartoutInset))
	case printing.TemplatePolaroid:
		return ui.Image().URI(uri).ObjectFit(ui.FitCover).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).
			Padding(ui.Padding{Top: polaroidSide, Left: polaroidSide, Right: polaroidSide, Bottom: polaroidFoot})
	default:
		return ui.Image().URI(uri).ObjectFit(ui.FitCover).Frame(ui.Frame{}.Size(ui.Full, ui.Full))
	}
}
