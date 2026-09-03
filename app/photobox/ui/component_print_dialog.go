package uiphotobox

import (
	"fmt"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/app/photo"
	preview "github.com/torbenschinke/eventprint/app/photobox/ui/preview"
	"github.com/torbenschinke/eventprint/app/printing"
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
	img, width, height := d.previewImage()

	return ui.VStack(
		preview.Selector(img, width, height, d.template),
		ui.Text("Der Ausdruck erscheint innerhalb einer Minute am Drucker.").Font(ui.BodySmall),
	).Gap(ui.L24).Alignment(ui.Center)
}

// previewImage liefert das Bild samt seinen Maßen. Die Maße entscheiden, ob
// die Vorschau ein quer oder hochkant liegendes Blatt zeigt.
func (d printDialog) previewImage() (image.ID, int, int) {
	id := d.selected.Get()
	if id == "" {
		return "", 0, 0
	}

	optPhoto, err := d.opts.Photos.FindByID(d.wnd.Subject(), id)
	if err != nil || optPhoto.IsNone() {
		return "", 0, 0
	}

	p := optPhoto.Unwrap()

	return p.Image, p.Width, p.Height
}
