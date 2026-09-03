package uiphotobox

import (
	"strings"
	"time"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/app/photo"
)

// boothLatestCount ist die Anzahl der Fotos auf dem Startbildschirm. Mehr
// passen auf einem üblichen Fotobox-Display nicht lesbar nebeneinander, und
// die vollständige Historie erreicht man ohnehin über das Menü.
const boothLatestCount = 12

// boothRefreshRate bestimmt, wie schnell ein neues Kamerabild auf dem
// Startbildschirm erscheint. Der Bildschirm wird von niemandem bedient,
// während die Kamera auslöst, deshalb wird zyklisch neu gezeichnet statt auf
// Events zu warten.
const boothRefreshRate = 3 * time.Second

// PageBooth ist der Startbildschirm der Fotobox: links der QR-Code zum
// Hochladen, rechts die zuletzt entstandenen Bilder. Ein Tippen auf ein Bild
// öffnet die Layout-Auswahl und druckt es erneut.
func PageBooth(wnd core.Window, opts Options) core.View {
	dialog := newPrintDialog(wnd, opts)
	pad := newPinPad(wnd, opts.Pin)

	photos, err := opts.Photos.FindLatest(wnd.Subject(), boothLatestCount)
	if err != nil {
		return alert.BannerError(err)
	}

	title := ""
	if opts.EventTitle != nil {
		title = opts.EventTitle()
	}

	if title == "" {
		title = "Willkommen an der Fotobox"
	}

	content := ui.VStack(
		alert.BannerMessages(wnd),
		dialog.View(),
		pad.View(),

		ui.Text(title).Font(ui.DisplaySmall),
		ui.Text("Tippe auf ein Bild, um es noch einmal zu drucken.").Font(ui.BodyLarge),

		ui.HStack(
			uploadInvitation(wnd, opts, pad),
			latestPhotos(photos, dialog),
		).
			Gap(ui.L40).
			Alignment(ui.Top).
			FullWidth(),
	).
		Gap(ui.L24).
		Alignment(ui.Center).
		WithPadding(ui.Padding{}.All(ui.L32)).
		Frame(ui.Frame{}.FullWidth())

	return ui.RedrawAtFixedRate(wnd, boothRefreshRate, content)
}

// uploadInvitation zeigt den QR-Code, über den Gäste eigene Bilder beisteuern.
func uploadInvitation(wnd core.Window, opts Options, pad pinPad) core.View {
	url := ""
	if opts.UploadURL != nil {
		url = opts.UploadURL()
	}

	return ui.VStack(
		ui.Text("Eigene Fotos drucken").Font(ui.TitleMedium),
		ui.Text("Code scannen, Bild auswählen, fertig.").Font(ui.BodyMedium).
			TextAlignment(ui.TextAlignCenter),
		configureGate(wnd, opts, pad,
			ui.QrCode(url).
				AccessibilityLabel("QR-Code zum Hochladen eigener Fotos").
				Frame(ui.Frame{}.Size(ui.L256, ui.L256)),
		),
		ui.Text(url).Font(ui.MonoSmall).
			TextAlignment(ui.TextAlignCenter),
		unreachableURLHint(wnd, opts, url),
	).
		Gap(ui.L16).
		Alignment(ui.Center).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Border(ui.Border{}.Radius(ui.L16)).
		Frame(ui.Frame{Width: ui.L400})
}

// localHosts sind die Namen, unter denen die Fotobox nur auf dem eigenen
// Rechner erreichbar ist.
var localHosts = []string{"localhost", "127.0.0.1", "[::1]", "0.0.0.0"}

// unreachableURLHint warnt, wenn im QR-Code eine Adresse steht, die vom
// Smartphone eines Gastes nicht erreichbar ist.
//
// Das passiert regelmäßig hinter einem Reverse Proxy oder wenn die Fotobox
// zuerst lokal geöffnet wurde: Nago leitet den öffentlichen Namen sonst aus
// der ersten Verbindung ab. Der Hinweis richtet sich an die Bedienung, nicht
// an die Gäste, und erscheint deshalb nur für angemeldete Nutzer.
func unreachableURLHint(wnd core.Window, opts Options, url string) core.View {
	if !canConfigure(wnd, opts) || opts.BoothSettings == "" {
		return nil
	}

	reachable := true
	for _, host := range localHosts {
		if strings.Contains(url, "//"+host) {
			reachable = false
			break
		}
	}

	if reachable {
		return nil
	}

	return ui.VStack(
		ui.Text("Diese Adresse erreicht nur dieser Rechner. Gäste können den Code nicht nutzen.").
			Font(ui.BodySmall).
			TextAlignment(ui.TextAlignCenter),
		ui.SecondaryButton(func() {
			wnd.Navigation().ForwardTo(opts.BoothSettings, opts.BoothSettingsParams)
		}).Title("Öffentliche Adresse setzen"),
	).Gap(ui.L8).Alignment(ui.Center)
}

// latestPhotos rendert das Raster der jüngsten Aufnahmen.
func latestPhotos(photos []photo.Photo, dialog printDialog) core.View {
	if len(photos) == 0 {
		return emptyHint(
			"Noch keine Fotos",
			"Sobald die Kamera auslöst oder ein Gast ein Bild hochlädt, erscheint es hier.",
		)
	}

	var cells []ui.TGridCell
	for _, p := range photos {
		cells = append(cells, ui.GridCell(photoTile(p, ui.L160, func() {
			dialog.Open(p.ID)
		})))
	}

	return ui.Grid(cells...).
		Columns(4).
		Gap(ui.L16).
		FullWidth()
}

// configureGate macht den QR-Code zur verborgenen Tuer in die Einrichtung.
//
// Fuenfmal zuegig antippen oeffnet das Tastenfeld. Ein sichtbarer Knopf waere
// ehrlicher, aber er stuende den ganzen Abend vor Gaesten, und jeder
// neugierige Fehlversuch sperrte den Betreuer fuer Sekunden aus. Der QR-Code
// ist die richtige Flaeche dafuer: Gaeste scannen ihn, sie tippen ihn nicht.
func configureGate(wnd core.Window, opts Options, pad pinPad, content core.View) core.View {
	if opts.Pin.Tap == nil {
		return content
	}

	return ui.VStack(content).
		Action(func() {
			if opts.Pin.Tap(string(wnd.Session().ID())) {
				pad.Open()
			}
		})
}

// canConfigure beantwortet die Frage, um die es wirklich geht.
func canConfigure(wnd core.Window, opts Options) bool {
	if opts.CanConfigure == nil {
		return false
	}

	return opts.CanConfigure(wnd)
}
