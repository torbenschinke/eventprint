// Package preview renders the print-template selector shared by photobox and photoupld.
package preview

import (
	"go.wdy.de/nago/application/image"
	httpimage "go.wdy.de/nago/application/image/http"
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"

	"github.com/torbenschinke/eventprint/printing"
)

const (
	paperWhite    ui.Color = "#FFFFFF"
	paperWidth             = ui.L96
	paperHeight            = ui.Length("9rem")
	borderInset            = ui.Length("0.45rem")
	polaroidSide           = ui.Length("0.55rem")
	polaroidFoot           = ui.Length("2rem")
	previewPixels          = 256
)

// Selector displays all templates using a prepared Nago image pyramid.
func Selector(img image.ID, selected *core.State[printing.TemplateID]) core.View {
	var cards []core.View
	for _, tpl := range printing.Templates() {
		cards = append(cards, card(img, tpl, selected))
	}

	return ui.HStack(cards...).Gap(ui.L16).Alignment(ui.Top).Wrap(true)
}

func card(img image.ID, tpl printing.Template, selected *core.State[printing.TemplateID]) core.View {
	borderColor := ui.M5
	if selected.Get() == tpl.ID {
		borderColor = ui.A0
	}

	return ui.VStack(
		Paper(img, tpl.ID),
		ui.Text(tpl.Name).Font(ui.LabelLarge),
		ui.Text(tpl.Description).Font(ui.BodySmall).TextAlignment(ui.TextAlignCenter),
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		Action(func() { selected.Set(tpl.ID) }).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12).Width(ui.L2).Color(borderColor)).
		Frame(ui.Frame{Width: ui.L200})
}

// Paper renders the selected image in the proportions of a 10x15 cm print.
func Paper(img image.ID, tpl printing.TemplateID) core.View {
	return ui.VStack(motif(img, tpl)).
		Alignment(ui.Center).
		BackgroundColor(paperWhite).
		Border(ui.Border{}.Radius(ui.L4).Width(ui.L1).Color(ui.M5)).
		Frame(ui.Frame{Width: paperWidth, Height: paperHeight})
}

func motif(img image.ID, tpl printing.TemplateID) core.View {
	if img == "" {
		return nil
	}

	// FitCover selects a suitable precomputed pyramid element instead of the original.
	uri := httpimage.URI(img, image.FitCover, previewPixels, previewPixels)
	switch tpl {
	case printing.TemplateBorder:
		return ui.Image().URI(uri).ObjectFit(ui.FitContain).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).Padding(ui.Padding{}.All(borderInset))
	case printing.TemplatePolaroid:
		return ui.Image().URI(uri).ObjectFit(ui.FitCover).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).
			Padding(ui.Padding{Top: polaroidSide, Left: polaroidSide, Right: polaroidSide, Bottom: polaroidFoot})
	default:
		return ui.Image().URI(uri).ObjectFit(ui.FitCover).Frame(ui.Frame{}.Size(ui.Full, ui.Full))
	}
}
