// Package uiwifi enthält die Oberfläche der Funkverbindung.
package uiwifi

import (
	"context"
	"errors"
	"fmt"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"
	"go.wdy.de/nago/presentation/ui/progress"

	"github.com/torbenschinke/eventprint/app/wifi"
)

// Options bündelt, was die Seite zur Arbeit braucht.
type Options struct {
	// WiFi sind die Anwendungsfälle der Funkverbindung.
	WiFi wifi.UseCases
}

// PageWiFi ist die Seite zum Einrichten der Funkverbindung.
//
// Die Fotobox wird an fremden Orten aufgebaut, und das Funknetz ist dort jedes
// Mal ein anderes. Wer sie aufbaut, soll das ohne SSH erledigen können.
//
// Die Tastatur des Raspberry Pi 400 ist angeschlossen und wird hier bewusst
// benutzt: Ein WPA-Kennwort ist lang und enthält Sonderzeichen; ein
// selbstgebautes Tastenfeld wäre dafür eine Zumutung. Die PIN hat eines, weil
// sie aus sechs Ziffern besteht – hier liegt der Fall anders.
func PageWiFi(wnd core.Window, opts Options) core.View {
	m := newModel(wnd, opts)

	// Beim Betreten genau einmal laden. Ohne den Merker liefe bei jedem
	// Neuzeichnen eine neue Suche an – und eine Suche belegt das Funkgerät
	// mehrere Sekunden.
	m.loadOnce()

	return ui.VStack(
		alert.BannerMessages(wnd),
		m.passwordDialog(),

		ui.Text("Funkverbindung").Font(ui.Title),

		m.currentConnection(),
		m.networkList(),
	).
		Gap(ui.L16).
		Alignment(ui.Leading).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Frame(ui.Frame{}.FullWidth())
}

// model hält die Zustände der Seite beisammen.
type model struct {
	wnd  core.Window
	opts Options

	started  *core.State[bool]
	loading  *core.State[bool]
	networks *core.State[[]wifi.Network]
	status   *core.State[wifi.Status]
	loadErr  *core.State[string]

	selected   *core.State[string]
	password   *core.State[string]
	askPass    *core.State[bool]
	connErr    *core.State[string]
	connecting *core.State[bool]
}

func newModel(wnd core.Window, opts Options) *model {
	return &model{
		wnd:  wnd,
		opts: opts,

		started:  core.StateOf[bool](wnd, "wifi-started"),
		loading:  core.StateOf[bool](wnd, "wifi-loading"),
		networks: core.StateOf[[]wifi.Network](wnd, "wifi-networks"),
		status:   core.StateOf[wifi.Status](wnd, "wifi-status"),
		loadErr:  core.StateOf[string](wnd, "wifi-load-error"),

		selected:   core.StateOf[string](wnd, "wifi-selected"),
		password:   core.StateOf[string](wnd, "wifi-password"),
		askPass:    core.StateOf[bool](wnd, "wifi-ask-password"),
		connErr:    core.StateOf[string](wnd, "wifi-connect-error"),
		connecting: core.StateOf[bool](wnd, "wifi-connecting"),
	}
}

// loadOnce stösst die Suche beim ersten Zeichnen an.
func (m *model) loadOnce() {
	if m.started.Get() {
		return
	}

	m.started.Set(true)
	m.reload()
}

// reload sucht nebenläufig nach Funknetzen.
//
// Die Suche dauert mehrere Sekunden, weil das Funkgerät dafür die Kanäle
// abhört. Liefe sie im Zeichenpfad, stünde die Oberfläche so lange still.
func (m *model) reload() {
	if m.loading.Get() {
		return
	}

	m.loading.Set(true)
	m.loadErr.Set("")

	subject := m.wnd.Subject()
	ctx := context.Background()

	go func() {
		nets, err := m.opts.WiFi.Scan(subject, ctx)

		var status wifi.Status
		if err == nil {
			status, err = m.opts.WiFi.Current(subject, ctx)
		}

		// Post reiht die Änderung in die Ereignisschleife des Fensters ein.
		// Den Zustand direkt aus der Goroutine zu setzen wäre ein Wettlauf mit
		// dem Zeichnen; Post zeichnet ausserdem selbst neu.
		//
		// Der Rückgabewert ist false, wenn das Fenster inzwischen weg ist –
		// dann ist nichts mehr zu tun, und das ist kein Fehler.
		m.wnd.Post(func() {
			m.loading.Set(false)

			if err != nil {
				m.loadErr.Set(err.Error())
				return
			}

			m.networks.Set(nets)
			m.status.Set(status)
		})
	}()
}

// currentConnection zeigt, woran die Fotobox gerade ist.
func (m *model) currentConnection() core.View {
	status := m.status.Get()

	body := ui.Text("Nicht verbunden").Font(ui.BodyLarge)
	if status.Connected {
		body = ui.Text(fmt.Sprintf("Verbunden mit %s", status.SSID)).Font(ui.BodyLarge)
	}

	var strength core.View
	if status.Connected {
		strength = ui.HStack(
			signalBars(status.Bars()),
			ui.Text(fmt.Sprintf("%d %%", status.Signal)).Font(ui.BodySmall),
		).Gap(ui.L8)
	}

	return ui.VStack(
		ui.Text("Aktuelle Verbindung").Font(ui.TitleSmall),
		ui.HStack(body, ui.Spacer(), strength).FullWidth(),
	).
		Gap(ui.L8).
		Alignment(ui.Leading).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.FullWidth())
}

// networkList zeigt die gefundenen Netze.
func (m *model) networkList() core.View {
	header := ui.HStack(
		ui.Text("Verfügbare Netze").Font(ui.TitleSmall),
		ui.Spacer(),
		ui.SecondaryButton(m.reload).Title("Erneut suchen").Enabled(!m.loading.Get()),
	).FullWidth()

	if m.loading.Get() {
		return ui.VStack(
			header,
			ui.Text("Suche läuft. Das dauert einige Sekunden.").Font(ui.BodyMedium),
		).Gap(ui.L12).Alignment(ui.Leading).FullWidth()
	}

	if msg := m.loadErr.Get(); msg != "" {
		return ui.VStack(
			header,
			ui.Text("Die Suche ist fehlgeschlagen: "+msg).Font(ui.BodyMedium).Color(ui.SE0),
		).Gap(ui.L12).Alignment(ui.Leading).FullWidth()
	}

	nets := m.networks.Get()
	if len(nets) == 0 {
		return ui.VStack(
			header,
			ui.Text("Kein Funknetz empfangen.").Font(ui.BodyMedium),
		).Gap(ui.L12).Alignment(ui.Leading).FullWidth()
	}

	rows := []core.View{header}
	for _, n := range nets {
		rows = append(rows, m.networkRow(n))
	}

	return ui.VStack(rows...).Gap(ui.L8).Alignment(ui.Leading).FullWidth()
}

func (m *model) networkRow(n wifi.Network) core.View {
	right := "offen"
	if n.Secured {
		right = "gesichert"
	}

	if n.Active {
		right = "verbunden"
	}

	return ui.HStack(
		signalBars(n.Bars()),
		ui.Text(n.SSID).Font(ui.BodyLarge),
		ui.Spacer(),
		ui.Text(right).Font(ui.BodySmall),
	).
		Gap(ui.L12).
		BackgroundColor(ui.M3).
		Action(func() { m.choose(n) }).
		WithPadding(ui.Padding{}.All(ui.L12)).
		Border(ui.Border{}.Radius(ui.L8)).
		Frame(ui.Frame{}.FullWidth())
}

// choose öffnet die Kennwortabfrage oder verbindet direkt.
func (m *model) choose(n wifi.Network) {
	m.selected.Set(n.SSID)
	m.password.Set("")
	m.connErr.Set("")

	if !n.Secured {
		// Ein offenes Netz nach einem Kennwort zu fragen wäre nur verwirrend.
		m.connect()
		return
	}

	m.askPass.Set(true)
}

// passwordDialog fragt das WPA-Kennwort ab.
func (m *model) passwordDialog() core.View {
	// Bewusst ohne eigene Abfrage auf askPass: alert.TDialog.Render prueft das
	// selbst und liefert dann nil. Wer den Dialog stattdessen aus dem Baum
	// nimmt, nimmt den Knoepfen ihre Verankerung.
	body := ui.VStack(
		ui.Text(fmt.Sprintf("Kennwort für %s", m.selected.Get())).Font(ui.BodyMedium),

		// Ein Passwortfeld und kein Textfeld: Die Fotobox steht sichtbar im
		// Raum, und ein WPA-Kennwort gehört nicht offen auf einen Bildschirm,
		// den alle sehen.
		ui.PasswordField("WPA-Kennwort", m.password.Get()).
			InputValue(m.password).
			ID("wifi-password").
			FullWidth(),

		m.connectStatus(),

		// Der Hinweis gehört hierher, weil die Folge sonst wie ein Fehler
		// aussieht: Wer die Fotobox über das Netz bedient, verliert beim
		// Wechsel des Funknetzes genau die Verbindung, über die er gerade
		// klickt. Die Oberfläche friert dann ein, obwohl alles geklappt hat.
		ui.Text("Wird die Fotobox über das Netz bedient, bricht dabei die Verbindung zu dieser Seite ab. Das ist normal.").
			Font(ui.BodySmall),
	).Gap(ui.L12).Alignment(ui.Leading).Frame(ui.Frame{}.FullWidth())

	return alert.Dialog(
		"Mit Funknetz verbinden",
		body,
		m.askPass,
		alert.Width(ui.L480),
		alert.Cancel(nil),
		alert.Confirm(func() (close bool) {
			m.connect()

			// Offen lassen: Erst wenn die Verbindung steht, schliesst der
			// Dialog. Sonst verschwände er bei falschem Kennwort, und niemand
			// wüsste, warum nichts passiert ist.
			return false
		}),
	)
}

// connectStatus zeigt, woran der Verbindungsaufbau gerade ist.
//
// Ohne diese Anzeige war der Bestätigen-Knopf bis zu 45 Sekunden lang ohne
// jede sichtbare Wirkung: Der Aufbau läuft nebenläufig, der Dialog blieb
// unverändert stehen, und ein zweiter Klick lief in die Sperre gegen
// Doppelaufrufe. Von außen sah das aus wie ein toter Knopf.
func (m *model) connectStatus() core.View {
	if m.connecting.Get() {
		return ui.VStack(
			progress.LinearProgress().Frame(ui.Frame{}.FullWidth()),
			ui.Text("Verbindung wird hergestellt. Das dauert einige Sekunden.").
				Font(ui.BodySmall),
		).Gap(ui.L4).Alignment(ui.Leading).Frame(ui.Frame{}.FullWidth())
	}

	msg := m.connErr.Get()
	if msg == "" {
		return nil
	}

	return ui.Text(msg).Font(ui.BodySmall).Color(ui.SE0)
}

// connect verbindet nebenläufig.
func (m *model) connect() {
	if m.connecting.Get() {
		return
	}

	m.connecting.Set(true)
	m.connErr.Set("")

	subject := m.wnd.Subject()
	ssid := m.selected.Get()
	password := m.password.Get()
	ctx := context.Background()

	go func() {
		err := m.opts.WiFi.Connect(subject, ctx, ssid, password)

		m.wnd.Post(func() {
			m.connecting.Set(false)

			if err != nil {
				var wrong wifi.WrongPasswordError
				if errors.As(err, &wrong) {
					m.connErr.Set("Das Kennwort wurde nicht angenommen.")
				} else {
					m.connErr.Set(err.Error())
				}

				return
			}

			// Das Kennwort sofort vergessen; es wird nicht mehr gebraucht.
			m.password.Set("")
			m.askPass.Set(false)

			// Der neue Zustand ist erst nach einer Abfrage bekannt.
			m.reload()
		})
	}()
}

// signalBars zeichnet die Feldstärke als vier Balken.
//
// Vier Balken statt einer Prozentzahl: Vor Ort ist die einzige Frage, ob es
// hier reicht oder die Fotobox näher ans Funknetz muss. Auf 47 Prozent kann
// niemand antworten.
func signalBars(bars int) core.View {
	heights := []ui.Length{ui.L4, ui.L8, ui.L12, ui.L16}

	var cells []core.View
	for i, h := range heights {
		colour := ui.M6
		if i < bars {
			colour = ui.M8
		}

		cells = append(cells, ui.HStack().
			BackgroundColor(colour).
			Border(ui.Border{}.Radius(ui.L2)).
			Frame(ui.Frame{Width: ui.L4, Height: h}))
	}

	return ui.HStack(cells...).Gap(ui.L2).Alignment(ui.Bottom)
}
