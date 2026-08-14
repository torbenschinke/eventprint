package uiphotobox

import (
	"fmt"

	"go.wdy.de/nago/application/image"
	httpimage "go.wdy.de/nago/application/image/http"
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
)

// printDialog kapselt die Layout-Auswahl, die vor jedem Druck erscheint.
// Sie wird von Startbildschirm und Galerie gleichermaßen verwendet, damit ein
// Gast überall dieselbe Interaktion vorfindet.
type printDialog struct {
	wnd       core.Window
	opts      Options
	presented *core.State[bool]
	selected  *core.State[photo.ID]
	template  *core.State[printing.TemplateID]
}

// newPrintDialog legt die Zustände des Dialogs an. Der Aufruf gehört an den
// Anfang einer Seitenfunktion.
func newPrintDialog(wnd core.Window, opts Options) printDialog {
	return printDialog{
		wnd:       wnd,
		opts:      opts,
		presented: core.StateOf[bool](wnd, "print-dialog-presented"),
		selected:  core.StateOf[photo.ID](wnd, "print-dialog-photo"),
		template:  core.StateOf[printing.TemplateID](wnd, "print-dialog-template").Init(func() printing.TemplateID { return printing.TemplateFull }),
	}
}

// Open zeigt den Dialog für das übergebene Foto an.
func (d printDialog) Open(id photo.ID) {
	d.selected.Set(id)
	d.presented.Set(true)
}

// View liefert den Dialog. Er rendert nur etwas, solange er geöffnet ist, und
// gehört daher unverändert in jede Seite, die drucken können soll.
func (d printDialog) View() core.View {
	return alert.Dialog(
		"Wie soll gedruckt werden?",
		d.body(),
		d.presented,
		// XLarge, sonst schneidet der Dialog die dritte Layout-Karte ab.
		alert.XLarge(),
		alert.Cancel(nil),
		alert.Confirm(func() (close bool) {
			id := d.selected.Get()
			if id == "" {
				return true
			}

			if _, err := d.opts.Printing.Print(d.wnd.Subject(), id, d.template.Get()); err != nil {
				alert.ShowBannerError(d.wnd, err)
				return true
			}

			// Im Testbetrieb nicht "wird gedruckt" behaupten – sonst wartet
			// der Gast vergeblich am Drucker.
			if printing.IsTestMode(d.opts.Printing.Printer) {
				alert.ShowBannerMessage(d.wnd, alert.Message{
					Title:   "Testmodus",
					Message: "Es ist kein Drucker eingerichtet, deshalb wurde nichts ausgedruckt.",
					Intent:  alert.IntentWarning,
				})

				return true
			}

			alert.ShowBannerMessage(d.wnd, alert.Message{
				Title:   "Wird gedruckt",
				Message: fmt.Sprintf("Das Foto liegt als %s in der Warteschlange von %q.", printing.TemplateByID(d.template.Get()).Name, d.opts.Printing.Printer.Name()),
				Intent:  alert.IntentOk,
			})

			return true
		}),
	)
}

func (d printDialog) body() core.View {
	var cards []core.View
	for _, tpl := range printing.Templates() {
		cards = append(cards, d.templateCard(tpl))
	}

	return ui.VStack(
		// Wrap, damit auf dem Smartphone eines Gastes untereinander
		// dargestellt wird, was auf dem Fotobox-Display nebeneinander passt.
		ui.HStack(cards...).Gap(ui.L16).Alignment(ui.Top).Wrap(true),
		ui.Text("Der Ausdruck erscheint innerhalb einer Minute am Drucker.").Font(ui.BodySmall),
	).Gap(ui.L24).Alignment(ui.Center)
}

// templateCard zeigt ein Layout mit einer Vorschau des tatsächlich gewählten
// Fotos. Das ist verbindlicher als ein abstraktes Schema: Ein Gast sieht
// sofort, welcher Teil seines Motivs beim formatfüllenden Druck wegfällt.
func (d printDialog) templateCard(tpl printing.Template) core.View {
	borderColor := ui.M5
	if d.template.Get() == tpl.ID {
		borderColor = ui.A0
	}

	return ui.VStack(
		d.templatePreview(tpl.ID),
		ui.Text(tpl.Name).Font(ui.LabelLarge),
		ui.Text(tpl.Description).Font(ui.BodySmall).
			TextAlignment(ui.TextAlignCenter),
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		Action(func() {
			d.template.Set(tpl.ID)
		}).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12).Width(ui.L2).Color(borderColor)).
		Frame(ui.Frame{Width: ui.L200})
}

// paperWhite ist die einzige fest verdrahtete Farbe der Oberfläche.
//
// Sie ist keine Design-Entscheidung, sondern bildet einen physischen
// Gegenstand ab: Das Papier im Drucker ist weiß, auch wenn die Anwendung im
// dunklen Farbschema läuft. Alles andere überlässt die Oberfläche dem Theme.
const paperWhite ui.Color = "#FFFFFF"

// Abmessungen der Papier-Miniatur im Verhältnis 2:3, passend zu 10x15 cm.
const (
	previewPaperWidth  = ui.L96
	previewPaperHeight = ui.Length("9rem")

	// Rand bzw. Steg der Miniatur, maßstäblich zum echten Ausdruck.
	previewBorderInset  = ui.Length("0.45rem")
	previewPolaroidSide = ui.Length("0.55rem")
	previewPolaroidFoot = ui.Length("2rem")
)

// templatePreview zeichnet das Papier als Miniatur und legt das gewählte Foto
// gemäß Layout darauf.
func (d printDialog) templatePreview(tpl printing.TemplateID) core.View {
	preview := ui.VStack(d.previewMotif(tpl)).
		Alignment(ui.Center).
		BackgroundColor(paperWhite).
		Border(ui.Border{}.Radius(ui.L4).Width(ui.L1).Color(ui.M5)).
		Frame(ui.Frame{Width: previewPaperWidth, Height: previewPaperHeight})

	return preview
}

// previewMotif liefert das Foto in der Fläche, die das jeweilige Layout ihm
// auf dem Papier zugesteht.
func (d printDialog) previewMotif(tpl printing.TemplateID) core.View {
	uri := d.previewURI()
	if uri == "" {
		return nil
	}

	switch tpl {
	case printing.TemplateBorder:
		// vollständiges Motiv, ringsum gleichmäßig Papier
		return ui.Image().
			URI(uri).
			ObjectFit(ui.FitContain).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).
			Padding(ui.Padding{}.All(previewBorderInset))

	case printing.TemplatePolaroid:
		// schmaler Rand oben und seitlich, breiter Steg unten
		return ui.Image().
			URI(uri).
			ObjectFit(ui.FitCover).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full)).
			Padding(ui.Padding{
				Top:    previewPolaroidSide,
				Left:   previewPolaroidSide,
				Right:  previewPolaroidSide,
				Bottom: previewPolaroidFoot,
			})

	default: // TemplateFull
		// Motiv füllt das Papier, der Überstand wird beschnitten
		return ui.Image().
			URI(uri).
			ObjectFit(ui.FitCover).
			Frame(ui.Frame{}.Size(ui.Full, ui.Full))
	}
}

// previewURI liefert die Adresse einer kleinen Variante des gewählten Fotos.
// Es wird bewusst eine Miniatur angefordert – für 96 Pixel Breite muss nicht
// das mehrere Megabyte große Original über die Leitung.
func (d printDialog) previewURI() core.URI {
	id := d.selected.Get()
	if id == "" {
		return ""
	}

	optPhoto, err := d.opts.Photos.FindByID(d.wnd.Subject(), id)
	if err != nil || optPhoto.IsNone() {
		return ""
	}

	return httpimage.URI(optPhoto.Unwrap().Image, image.FitNone, previewPx, previewPx)
}

// previewPx ist die angeforderte Kantenlänge der Miniatur im Dialog.
const previewPx = 256
