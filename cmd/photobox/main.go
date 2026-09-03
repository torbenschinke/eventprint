// Command photobox ist die Fotobox-Anwendung für Hochzeiten, Jubiläen und
// ähnliche Veranstaltungen.
//
// Die Anwendung läuft auf dem Rechner, an dem Kamera und Fotodrucker per USB
// hängen. Sie zeigt auf dem Fotobox-Display die zuletzt entstandenen Bilder,
// erlaubt Gästen per QR-Code eigene Fotos beizusteuern und druckt beides in
// wählbaren Layouts auf 10x15 cm.
//
// Konfiguration erfolgt über Umgebungsvariablen, siehe
// [cfgphotobox.OptionsFromEnv]. Ein typischer Start:
//
//	HOST=0.0.0.0 \
//	HOSTNAME=fotobox.local:3000 \
//	EVENTPRINT_TITLE="Hochzeit von Anna & Ben" \
//	EVENTPRINT_PRINTER=CZ01 \
//	NAGO_COOKIES_INSECURE=true \
//	go run ./cmd/photobox
//
// #[go.permission.generateTable]
package main

import (
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application"
	"go.wdy.de/nago/pkg/std"
	icons "go.wdy.de/nago/presentation/icons/hero/outline"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/web/vuejs"

	cfgphotobox "github.com/torbenschinke/eventprint/app/photobox/cfg"
)

func main() {
	application.Configure(func(cfg *application.Configurator) {
		cfg.SetApplicationID("de.torbenschinke.eventprint")
		cfg.Serve(vuejs.Dist())

		// Die Fotobox soll aus dem WLAN der Veranstaltung erreichbar sein,
		// sonst kann kein Gast den QR-Code nutzen.
		cfg.SetHost("0.0.0.0")

		option.MustZero(cfg.StandardSystems())

		// Für die Betreuung der Veranstaltung: ein zeitlich begrenzter
		// Administrator, über den sich Historie und Druckstatus verwalten
		// lassen. Gäste brauchen keine Anmeldung.
		std.Must(std.Must(cfg.UserManagement()).UseCases.EnableBootstrapAdmin(
			time.Now().Add(24*time.Hour),
			"%6UbRsCuM8N$auy",
		))

		opts := cfgphotobox.OptionsFromEnv()
		photobox := std.Must(cfgphotobox.Enable(cfg, opts))

		// Die Menueleiste gehoert dem Betreuer, nicht den Gaesten.
		//
		// Fuer die Veranstaltung reicht der Startbildschirm: Fotos ansehen,
		// antippen, drucken. Alles andere ist Betreuung und hat auf einem
		// 1024x600-Bildschirm nichts verloren, den den ganzen Abend Gaeste
		// bedienen. Jeder Eintrag verlangt deshalb PermConfigure.
		//
		// Gaeste bekommen den Rahmen gar nicht erst - das entscheidet
		// cfgphotobox.Enable je Seite. Deshalb kann Login hier eingeschaltet
		// bleiben, ohne dass ein Gast je eine Anmeldung zu sehen bekaeme.
		//
		// Login(true) ist dabei nicht verhandelbar: Es steuert nicht nur den
		// Anmelden-Eintrag, sondern den gesamten Profilbereich - und darin
		// haengt der Zugang zum Admin-Center. Mit Login(false) war das
		// Admin-Center unerreichbar, und ein selbstgebauter Abmelden-Knopf
		// verdeckte den Verlust, statt ihn zu beheben.
		cfg.SetDecorator(cfg.NewScaffold().
			Login(true).
			Logo(ui.Image().Embed(icons.Camera).Frame(ui.Frame{}.Size(ui.L48, ui.L48))).
			MenuEntry().Title("Fotobox").Icon(icons.Photo).Forward(photobox.Pages.Booth).OneOf(cfgphotobox.PermConfigure).
			MenuEntry().Title("Alle Fotos").Icon(icons.RectangleStack).Forward(photobox.Pages.Gallery).OneOf(cfgphotobox.PermConfigure).
			MenuEntry().Title("Druckstatus").Icon(icons.Printer).Forward(photobox.Pages.Jobs).OneOf(cfgphotobox.PermConfigure).
			MenuEntry().Title("Hochladen").Icon(icons.QrCode).Forward(photobox.Pages.Upload).OneOf(cfgphotobox.PermConfigure).
			Breakpoint(1000).
			Decorator())
	}).Run()
}
