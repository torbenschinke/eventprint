package uiphotobox

import (
	"fmt"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/photo"
)

// PageGallery zeigt die vollständige Historie aller Fotos – sowohl die von
// der Kamera als auch die von Gästen hochgeladenen.
func PageGallery(wnd core.Window, opts Options) core.View {
	dialog := newPrintDialog(wnd, opts)

	var (
		cells []ui.TGridCell
		count int
	)

	for p, err := range opts.Photos.FindAll(wnd.Subject()) {
		if err != nil {
			return alert.BannerError(err)
		}

		count++
		cells = append(cells, ui.GridCell(galleryTile(wnd, opts, p, dialog)))
	}

	if count == 0 {
		return ui.VStack(
			alert.BannerMessages(wnd),
			emptyHint("Noch keine Fotos", "Die Historie füllt sich, sobald das erste Bild entsteht."),
		).Frame(ui.Frame{}.FullWidth())
	}

	return ui.VStack(
		alert.BannerMessages(wnd),
		dialog.View(),

		ui.HStack(
			ui.Text("Alle Fotos").Font(ui.TitleLarge),
			ui.Spacer(),
			ui.Text(fmt.Sprintf("%d Bilder", count)).Font(ui.BodyMedium),
		).FullWidth(),

		ui.Grid(cells...).
			Columns(5).
			Gap(ui.L16).
			FullWidth(),
	).
		Gap(ui.L24).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Frame(ui.Frame{}.FullWidth())
}

// galleryTile ergänzt die reine Kachel um Herkunft, Zeitpunkt und die
// Möglichkeit, ein Bild wieder zu entfernen.
func galleryTile(wnd core.Window, opts Options, p photo.Photo, dialog printDialog) core.View {
	return ui.VStack(
		photoTile(p, ui.L160, func() {
			dialog.Open(p.ID)
		}),

		ui.Text(p.CreatedAt.Format("02.01.2006 15:04")).Font(ui.BodySmall),

		ui.Text(p.Source.String()).Font(ui.LabelSmall),

		ui.HStack(
			ui.SecondaryButton(func() {
				dialog.Open(p.ID)
			}).Title("Drucken"),

			ui.TertiaryButton(func() {
				if err := opts.Photos.Delete(wnd.Subject(), p.ID); err != nil {
					alert.ShowBannerError(wnd, err)
					return
				}

				alert.ShowBannerMessage(wnd, alert.Message{
					Title:  "Gelöscht",
					Intent: alert.IntentOk,
				})
			}).Title("Löschen"),
		).Gap(ui.L4),
	).Gap(ui.L4).Alignment(ui.Center)
}
