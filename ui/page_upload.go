package uiphotobox

import (
	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/photo"
)

// PageUpload ist die Seite, die ein Gast nach dem Scannen des QR-Codes auf
// seinem Smartphone sieht.
//
// Sie ist absichtlich auf einen einzigen Ablauf reduziert: Bild auswählen,
// Layout wählen, drucken. Es gibt keine Anmeldung, kein Konto und keine
// Navigation – auf einer Feier hat niemand Lust auf mehr als zwei Klicks.
func PageUpload(wnd core.Window, opts Options) core.View {
	dialog := newPrintDialog(wnd, opts)
	uploaded := core.AutoState[[]photo.Photo](wnd)

	return ui.VStack(
		alert.BannerMessages(wnd),
		dialog.View(),

		ui.Text("Dein Foto drucken").Font(ui.TitleLarge),
		ui.Text("Wähle ein Bild aus deiner Galerie. Es wird auf 10x15 cm gedruckt und erscheint auch auf der Fotobox.").Font(ui.BodyMedium).
			TextAlignment(ui.TextAlignCenter),

		ui.PrimaryButton(func() {
			importFromDevice(wnd, opts, uploaded)
		}).Title("Foto auswählen"),

		uploadResult(uploaded.Get(), dialog),
	).
		Gap(ui.L16).
		Alignment(ui.Center).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Frame(ui.Frame{}.FullWidth())
}

// importFromDevice öffnet den Dateidialog des Smartphones und übernimmt die
// ausgewählten Bilder.
//
// Der Aufruf muss aus einer Aktion heraus erfolgen, nicht aus der
// Render-Schleife – Nago erzwingt das.
func importFromDevice(wnd core.Window, opts Options, uploaded *core.State[[]photo.Photo]) {
	wnd.ImportFiles(core.ImportFilesOptions{
		ID:               "photobox-guest-upload",
		Multiple:         true,
		AllowedMimeTypes: []string{"image/jpeg", "image/png"},
		OnCompletion: func(files []core.File) {
			// Abbruch im Dateidialog des Browsers
			if len(files) == 0 {
				return
			}

			var imported []photo.Photo
			for _, file := range files {
				p, err := opts.Photos.Import(wnd.Subject(), photo.Options{Source: photo.SourceUpload}, file)
				if err != nil {
					alert.ShowBannerError(wnd, err)
					continue
				}

				imported = append(imported, p)
			}

			if len(imported) == 0 {
				return
			}

			uploaded.Set(append(imported, uploaded.Get()...))
			uploaded.Notify()
		},
	})
}

// uploadResult zeigt die soeben hochgeladenen Bilder mit einem Druck-Button.
func uploadResult(photos []photo.Photo, dialog printDialog) core.View {
	if len(photos) == 0 {
		return nil
	}

	var views []core.View
	views = append(views, ui.Text("Hochgeladen – jetzt drucken:").Font(ui.LabelLarge))

	for _, p := range photos {
		views = append(views, ui.VStack(
			photoTile(p, ui.L200, func() {
				dialog.Open(p.ID)
			}),
			ui.PrimaryButton(func() {
				dialog.Open(p.ID)
			}).Title("Drucken"),
		).Gap(ui.L8).Alignment(ui.Center))
	}

	return ui.VStack(views...).Gap(ui.L16).Alignment(ui.Center)
}
