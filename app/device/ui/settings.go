package uidevice

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/nas"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/app/wifi"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// Abschnitte der Einstellungen.
const (
	sectionWifi = iota
	sectionDisplay
	sectionPrinter
	sectionUpload
	sectionSources
	sectionKiosk
	sectionStorage
	sectionInfo
)

type sectionSpec struct {
	id    int
	label string
	sym   ui.Symbol
	face  ui.Color
}

var sections = []sectionSpec{
	{sectionWifi, "WLAN", outline.Globe, blue},
	{sectionDisplay, "Anzeige", outline.Sun, ui.RGB(0x2F, 0x6F, 0xE0)},
	{sectionPrinter, "Drucker & Papier", outline.Printer, grey},
	{sectionUpload, "Handy-Upload", outline.MobilePhone, green},
	{sectionSources, "Konten & Quellen", outline.CloudArrowUp, purple},
	{sectionKiosk, "Kiosk-Modus", outline.WandMagicSparkles, pink},
	{sectionStorage, "Speicher & USB", outline.ArchiveArrowDown, grey},
	{sectionInfo, "Info", outline.InfoCircle, grey},
}

// settingsScreen ist die geteilte Ansicht der Einstellungen.
func (a *App) settingsScreen(ctx *gift.Context, st *states) gift.View {
	current := ctx.Read(st.settings)
	rev := ctx.State("rev", 0)

	// Auf kleinen Panels sind Liste und Detail zwei Seiten wie am iPhone;
	// open sagt, ob gerade das Detail vorne liegt.
	open := st.settingsOpen
	ctx.Read(open)

	res := xgift.UseResource[device.Settings](ctx, "settings")
	res.LoadKeyed(ctx.Read(rev), func() (device.Settings, error) { return a.dev.Device.LoadSettings(a.dev.Subject()) })
	s := res.Value()

	save := func(fn func(*device.Settings)) {
		if _, err := a.dev.Device.SaveSettings(a.dev.Subject(), fn); !a.fail(err) {
			rev.Set(rev.Get() + 1)
		}
	}

	// Wie die Einstellungen von iPadOS: die Zeilen ohne eigene Fläche, die
	// Auswahl eine neutrale Kapsel.
	rows := []gift.View{}
	for _, sec := range sections {
		fg, face := ui.ColorLabel, ui.ColorClear
		if sec.id == current && !compact {
			face = ui.Fade(ui.ColorLabel, 0.08)
		}

		style := ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: capsule(46)}
		rows = append(rows, ui.Button(ui.HStack(
			xgift.IconTile(sec.sym, sec.face, u(30)),
			ui.Text(sec.label).FontSize(u(16)).Foreground(fg).Flex(1),
			ui.Icon(outline.AngleRight).Size(u(16)).Foreground(fg),
		).Gap(u(12)).Align(geom.Center), func() {
			st.settings.Set(sec.id)
			open.Set(true)
		}).
			Style(style).HoverStyle(style).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.14), CornerRadius: capsule(46)}).
			PaddingInsets(geom.Insets{Left: u(12), Right: u(12)}).
			MinHeight(u(46)))
	}

	head := []gift.View{
		link("‹ Home", func() { st.screen.Set(ScreenHome) }),
		title("Einstellungen", 30),
		ui.HStack(
			xgift.IconTile(outline.Printer, ink, u(48)),
			ui.VStack(title("eventprint", 17), muted("Heimdrucker · Citizen CZ-01", 13)).Gap(u(2)),
		).Gap(u(14)).Align(geom.Center).Padding(u(12)).Background(ui.ColorSurface).CornerRadius(u(12)),
	}

	if compact {
		head = []gift.View{ui.HStack(
			link("‹ Home", func() { st.screen.Set(ScreenHome) }),
			title("Einstellungen", 20).Flex(1).Align(ui.AlignCenter),
			ui.Box().Frame(u(90), 1),
		).Align(geom.Center)}
	}

	sidebar := ui.VStack(ui.VStack(append(head, ui.VStack(rows...).Gap(u(2)))...).
		Gap(u(pick(12, 6))).Padding(u(pick(16, 10))).
		Background(ui.ColorSurface).CornerRadius(u(pick(26, 18))).Shadow(paneShadow())).
		PaddingInsets(geom.Insets{Left: u(pick(12, 6)), Top: u(pick(12, 6)), Bottom: u(pick(12, 6))})

	var detail gift.View
	key := fmt.Sprint("detail", current)
	switch current {
	case sectionWifi:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.wifiSettings(ctx, st) })
	case sectionDisplay:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.displaySettings(s, save) })
	case sectionPrinter:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.printerSettings(ctx, st, s, save) })
	case sectionUpload:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.uploadSettings(ctx, st, s, save) })
	case sectionSources:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.sourceSettings(ctx, st, s, save) })
	case sectionKiosk:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.kioskSettings(ctx, st, s, save) })
	case sectionStorage:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.storageSettings(ctx, st, s) })
	default:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.infoSettings(ctx, st) })
	}

	name := ""
	for _, sec := range sections {
		if sec.id == current {
			name = sec.label
		}
	}

	// Auf großen Bildschirmen eine mittige Spalte, damit Zeilen nicht über
	// die ganze Breite laufen; auf kleinen die volle Breite.
	column := ui.VStack(detail, ui.Box().Frame(1, ui.OnScreenKeyboardHeight()))
	if !compact {
		column = column.MaxWidth(u(760))
	}

	scroller := ui.VScroll(ui.VStack(column).Align(geom.Top).
		PaddingInsets(geom.Insets{Left: u(pick(24, 16)), Right: u(pick(24, 16)), Bottom: u(24)})).Flex(1)

	if compact {
		list := xgift.Page{Key: "list", Depth: 0, View: xgift.Fill(ui.VScroll(sidebar)).Key("list")}
		page := xgift.Page{Key: "detail", Depth: 1, View: ui.VStack(
			ui.HStack(
				link("‹ Einstellungen", func() { open.Set(false) }),
				title(name, 17).Flex(1).Align(ui.AlignCenter),
				ui.Box().Frame(u(120), 1),
			).Align(geom.Center).PaddingInsets(geom.Insets{Left: u(10), Right: u(10), Top: u(10)}).MinHeight(u(44)),
			scroller,
		).Background(ui.ColorBackground)}

		if open.Get() {
			return xgift.Pages(page, list)
		}

		return xgift.Pages(list, page)
	}

	return xgift.HStretch(
		xgift.Fill(ui.VScroll(sidebar)).Width(u(400)),
		grow(ui.VStack(
			ui.HStack(fill(), title(name, 17), fill()).PaddingInsets(geom.Insets{Top: u(16), Bottom: u(8)}),
			scroller,
		).Background(ui.ColorBackground)),
	).Flex(1)
}

// --- WLAN -------------------------------------------------------------------

type wifiData struct {
	current  wifi.Status
	networks []wifi.Network
}

func (a *App) wifiSettings(ctx *gift.Context, st *states) gift.View {
	scan := ctx.State("scan", 0)
	res := xgift.UseResource[wifiData](ctx, "wifi")
	res.LoadKeyed(ctx.Read(scan), func() (wifiData, error) {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var d wifiData
		var err error
		if d.current, err = a.dev.WiFi.Current(a.dev.Subject(), c); err != nil {
			return d, err
		}

		d.networks, err = a.dev.WiFi.Scan(a.dev.Subject(), c)
		return d, err
	})

	d := res.Value()
	cur := "Nicht verbunden"
	if d.current.Connected {
		cur = fmt.Sprintf("%s · Empfang %d %%", d.current.SSID, d.current.Signal)
	}

	rows := []gift.View{}
	for _, n := range d.networks {
		label := n.SSID
		if n.Active {
			label += "  ✓"
		}

		lock := ""
		if n.Secured {
			lock = "gesichert"
		}

		rows = append(rows, ui.Row(label).Subtitle(fmt.Sprintf("Empfang %d %%", n.Signal)).Value(lock).
			Chevron(outline.AngleRight).
			OnTap(func() {
				st.wifiSSID.Set(n.SSID)
				if n.Secured {
					a.openSheet(SheetWifiPassword)
					return
				}

				a.connectWifi(n.SSID, "")
			}).Key(n.SSID))
	}

	status := "Netze werden gesucht …"
	switch {
	case res.Err() != nil:
		status = humane(res.Err())
	case res.Loaded() && len(rows) == 0:
		status = "Keine Netze gefunden."
	case res.Loaded():
		status = ""
	}

	items := []gift.View{
		section("VERBINDUNG", ui.Row("Aktuelles Netz").Value(cur)),
		ui.HStack(muted(status, 14).Flex(1), secondary("Erneut suchen", func() { scan.Set(scan.Get() + 1) })).Align(geom.Center),
	}

	if len(rows) > 0 {
		items = append(items, section("VERFÜGBARE NETZE", rows...))
	}

	return ui.VStack(items...).Gap(u(18))
}

// connectWifi verbindet im Hintergrund, denn nmcli braucht Sekunden.
func (a *App) connectWifi(ssid, password string) {
	a.show("Verbinde mit " + ssid + " …")
	go func() {
		c, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		err := a.dev.WiFi.Connect(a.dev.Subject(), c, ssid, password)
		xgift.Post(func() {
			if !a.fail(err) {
				a.show("Verbunden mit " + ssid + ".")
			}
		})
	}()
}

// wifiPasswordSheet fragt das Kennwort eines gesicherten Netzes ab.
func (a *App) wifiPasswordSheet(ctx *gift.Context, st *states) gift.View {
	ssid := ctx.Read(st.wifiSSID)
	ed := ui.Editor(ctx, "wifipw", "")

	connect := func() {
		pw := ed.Text()
		ed.SetText("")
		a.dismissSheet()
		a.connectWifi(ssid, pw)
	}

	return sheetCard(u(560),
		title("Kennwort für „"+ssid+"“", 22),
		ui.TextField(ed).Placeholder("WLAN-Kennwort").FontSize(u(18)).MinHeight(u(52)).OnSubmit(func(string) { connect() }),
		ui.HStack(secondary("Abbrechen", func() { a.dismissSheet() }), primary("Verbinden", connect).Flex(1)).Gap(u(12)),
	)
}

// --- Anzeige ----------------------------------------------------------------

// displaySettings wählt hell, dunkel oder automatisch und zeigt, wie die
// Oberfläche den Bildschirm bemessen hat – nützlich, wenn ein Panel seine
// Größe falsch meldet.
func (a *App) displaySettings(s device.Settings, save func(func(*device.Settings))) gift.View {
	modes := []device.Appearance{device.AppearanceAuto, device.AppearanceLight, device.AppearanceDark}
	current := slices.Index(modes, s.Appearance)
	if current < 0 {
		current = 0
	}

	d := a.screen
	size := fmt.Sprintf("%.0f × %.0f Punkte", d.viewport.W, d.viewport.H)
	if d.physical.W > 0 {
		size += fmt.Sprintf(" · %.0f × %.0f mm", d.physical.W, d.physical.H)
	}

	layout := "groß"
	if compact {
		layout = "kompakt"
	}

	return ui.VStack(
		section("ERSCHEINUNGSBILD",
			ui.Row("").Accessory(ui.SegmentedControl(current, []string{"Automatisch", "Hell", "Dunkel"}, func(i int) {
				save(func(s *device.Settings) { s.Appearance = modes[i] })
				a.refreshTheme()
			}).Capsule(true).FontSize(u(15)).Frame(geom.Unbounded(), u(40))),
		),
		muted("Automatisch ist die Box von 20 bis 7 Uhr dunkel. Der Kiosk ist immer dunkel.", 13).MaxLines(2).PaddingInsets(geom.Insets{Left: u(16)}),
		section("",
			ui.Row("Transparenz reduzieren").Subtitle("Deckende Flächen statt Glas auf Home und im Druck-Studio").
				Accessory(ui.Toggle(s.ReduceTransparency, func(v bool) {
					save(func(s *device.Settings) { s.ReduceTransparency = v })
					a.refreshTheme()
				})),
		),
		section("BILDSCHIRM",
			ui.Row("Fläche").Value(size),
			ui.Row("Vergrößerung").Value(fmt.Sprintf("%.2f × Dichte %v", scale, d.dens())),
			ui.Row("Anordnung").Value(layout),
		),
		muted("Die Oberfläche bemisst sich nach Auflösung und Größe des Panels. Wer anderes möchte, setzt EVENTPRINT_UI_SCALE in /etc/default/eventprint.", 13).MaxLines(3).PaddingInsets(geom.Insets{Left: u(16)}),
	).Gap(u(pick(18, 12)))
}

// --- Drucker ----------------------------------------------------------------

func (a *App) printerSettings(ctx *gift.Context, st *states, s device.Settings, save func(func(*device.Settings))) gift.View {
	queues := xgift.UseResource[[]string](ctx, "queues")
	queues.LoadOnce(func() ([]string, error) {
		c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		return a.dev.Queues.List(c)
	})

	chips := []gift.View{xgift.Chip("Testbetrieb", s.Printer.Queue == "", u(15), blue, func() {
		save(func(s *device.Settings) { s.Printer.Queue = "" })
	})}

	names := slices.Clone(queues.Value())
	if s.Printer.Queue != "" && !slices.Contains(names, s.Printer.Queue) {
		names = append(names, s.Printer.Queue)
	}

	for _, q := range names {
		chips = append(chips, xgift.Chip(strings.ReplaceAll(q, "_", " "), s.Printer.Queue == q, u(15), blue, func() {
			save(func(s *device.Settings) { s.Printer.Queue = q })
		}))
	}

	speed := 0
	if s.Printer.Speed() == printing.SpeedNormal {
		speed = 1
	}

	return ui.VStack(
		section("WARTESCHLANGE",
			ui.HScroll(chips...).Gap(u(8)).PaddingInsets(geom.Insets{Left: u(16), Right: u(16), Top: u(8), Bottom: u(8)}),
		),
		muted("Im Testbetrieb laufen Aufträge durch, ohne dass etwas gedruckt wird.", 13).PaddingInsets(geom.Insets{Left: u(16)}),
		section("DRUCK",
			ui.Row("Geschwindigkeit").Subtitle("Langsam färbt kräftiger").Accessory(
				ui.SegmentedControl(speed, []string{"Langsam", "Normal"}, func(i int) {
					save(func(s *device.Settings) {
						s.Printer.PrintSpeed = printing.SpeedLow
						if i == 1 {
							s.Printer.PrintSpeed = printing.SpeedNormal
						}
					})
				}).Capsule(true).FontSize(u(14)).Frame(u(220), u(36))),
			ui.Row("Ausschnitt auf Gesichter").Subtitle("Vorgabe für neue Druckvorgänge").Accessory(
				ui.Toggle(s.FaceCrop, func(v bool) { save(func(s *device.Settings) { s.FaceCrop = v }) })),
		),
		section("PAPIER",
			ui.Row("Vorrat").Value(fmt.Sprintf("%d von %d Blatt", s.PaperLeft, s.PaperCapacity)),
			ui.Row("Blatt je Set").Accessory(xgift.Stepper(s.PaperCapacity, 10, 200, u(16), func(v int) {
				save(func(s *device.Settings) { s.PaperCapacity = v })
			})),
			ui.Row("Neues Set eingelegt").Chevron(outline.AngleRight).OnTap(func() {
				if _, err := a.dev.Device.RefillPaper(a.dev.Subject()); !a.fail(err) {
					save(func(*device.Settings) {})
					a.show("Papiervorrat zurückgesetzt.")
				}
			}),
		),
		secondary("Druckaufträge ansehen", func() { st.screen.Set(ScreenJobs) }),
	).Gap(u(18))
}

// --- Handy-Upload -----------------------------------------------------------

func (a *App) uploadSettings(ctx *gift.Context, st *states, s device.Settings, save func(func(*device.Settings))) gift.View {
	tick := ctx.Read(st.tick)
	urlEd := ui.Editor(ctx, "relayurl", s.RelayURL)
	mailEd := ui.Editor(ctx, "relaymail", s.RelayAccount)
	tokenEd := ui.Editor(ctx, "relaytoken", "")

	// Die Kopplung: erst Mailadresse, dann Code. Der Stand gehört diesem
	// Bildschirm; wer ihn verlässt, fängt neu an.
	pending := ctx.State("pairing", relay.Pairing{})
	code := ctx.State("code", "")
	busy := ctx.State("busy", false)
	manual := ctx.State("manual", false)
	ctx.Read(pending)
	ctx.Read(busy)

	addr := xgift.UseResource[relay.Address](ctx, "addr")
	addr.LoadKeyed(tick/2, func() (relay.Address, error) { return a.dev.Relay.UploadAddress(a.dev.Subject(), true) })

	var qr gift.View = muted(orDash(addr.Value().Problem), 15).MaxLines(3)
	if link := addr.Value().URL; link != "" {
		qr = ui.HStack(xgift.QRCode(link, u(pick(140, 110))), muted(link, 13).MaxLines(4).Flex(1)).Gap(u(16)).Align(geom.Center)
	}

	request := func() {
		cmd := relay.BeginPairingCmd{URL: urlEd.Text(), Mail: mailEd.Text(), Device: s.EventTitle}
		busy.Set(true)
		go func() {
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			p, err := a.dev.Relay.BeginPairing(a.dev.Subject(), c, cmd)
			xgift.Post(func() {
				busy.Set(false)
				if a.fail(err) {
					return
				}

				code.Set("")
				pending.Set(p)
			})
		}()
	}

	confirm := func(entered string) {
		p := pending.Get()
		busy.Set(true)
		go func() {
			c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			outcome, err := a.dev.Relay.CompletePairing(a.dev.Subject(), c, p, entered)
			xgift.Post(func() {
				busy.Set(false)
				code.Set("")
				if a.fail(err) {
					return
				}

				switch outcome {
				case relay.PairingPaired:
					pending.Set(relay.Pairing{})
					save(func(*device.Settings) {})
					a.show("Verbunden mit " + p.Mail + ". Gäste können jetzt Fotos senden.")
				case relay.PairingExpired:
					pending.Set(relay.Pairing{})
					a.show("Der Code ist abgelaufen. Fordere einen neuen an.")
				case relay.PairingLocked:
					pending.Set(relay.Pairing{})
					a.show("Zu viele falsche Versuche. Fordere einen neuen Code an.")
				default:
					a.show("Der Code stimmt nicht. Bitte noch einmal.")
				}
			})
		}()
	}

	field := func(ed *ui.TextEditor, placeholder string) gift.View {
		return ui.TextField(ed).Placeholder(placeholder).FontSize(u(16)).Frame(u(pick(380, 300)), u(44))
	}

	var account []gift.View
	switch p := pending.Get(); {
	case p.ID != "":
		account = []gift.View{
			body("Wir haben einen Code an "+p.Mail+" geschickt – falls es dazu ein Konto gibt. Er gilt 30 Minuten.", 15).MaxLines(3),
			ui.HStack(fill(), xgift.PinPad(6, ctx.Read(code), u(pick(64, 50)), code.Set, confirm), fill()),
			ui.HStack(
				link("Andere Adresse", func() { pending.Set(relay.Pairing{}) }),
				fill(),
				link("Neuen Code anfordern", request),
			).Align(geom.Center),
		}
		if busy.Get() {
			account = append(account, muted("Einen Moment …", 14))
		}
	default:
		status := "Nicht verbunden"
		switch {
		case s.RelayToken != "" && s.RelayAccount != "":
			status = "Verbunden mit " + s.RelayAccount
		case s.RelayToken != "":
			status = "Verbunden mit einem von Hand eingetragenen Token"
		}

		label := "Code per Mail anfordern"
		if s.RelayToken != "" {
			label = "Mit anderem Konto koppeln"
		}

		if busy.Get() {
			label = "Wird angefordert …"
		}

		account = []gift.View{
			section("KONTO",
				ui.Row(status),
				ui.Row("Mailadresse").Accessory(field(mailEd, "du@example.de")),
			),
			muted("Trage die Adresse deines Kontos beim Upload-Dienst ein. Du bekommst einen sechsstelligen Code per Mail und tippst ihn hier ein – die Box holt sich ihren Zugang dann selbst.", 13).MaxLines(4).PaddingInsets(geom.Insets{Left: u(16)}),
			primary(label, request).Disabled(busy.Get()),
		}
	}

	rows := []gift.View{
		section("ZUSTAND", ui.Row("").Accessory(qr)),
		section("UPLOAD-DIENST", ui.Row("Adresse").Accessory(field(urlEd, "https://upload.example.de"))),
	}
	rows = append(rows, account...)

	// Der alte Weg bleibt, versteckt: für Dienste ohne Mailversand oder
	// wenn ein Administrator ein Token vorbereitet hat.
	if ctx.Read(manual) {
		rows = append(rows,
			section("TOKEN VON HAND", ui.Row("Token").Accessory(field(tokenEd, "Zugangstoken"))),
			primary("Token speichern", func() {
				url, token := strings.TrimSpace(urlEd.Text()), strings.TrimSpace(tokenEd.Text())
				save(func(s *device.Settings) { s.RelayURL, s.RelayToken, s.RelayAccount = url, token, "" })
				tokenEd.SetText("")
				a.show("Gespeichert. Die Verbindung wird neu aufgebaut.")
			}),
		)
	} else {
		rows = append(rows, link("Token von Hand eintragen", func() { manual.Set(true) }))
	}

	return ui.VStack(rows...).Gap(u(pick(18, 12)))
}

// --- Konten & Quellen -------------------------------------------------------

func (a *App) sourceSettings(ctx *gift.Context, st *states, s device.Settings, save func(func(*device.Settings))) gift.View {
	camera := "abgeschaltet"
	if a.dev.Camera != nil {
		cs := a.dev.Camera.Status()
		camera = "keine Kamera"
		if cs.Model != "" {
			camera = cs.Model
		}
	}

	return ui.VStack(
		gift.Component("nas", func(ctx *gift.Context) gift.View { return a.nasSettings(ctx, st, s, save) }),
		section("WEITERE QUELLEN",
			ui.Row("Handy-Upload per QR").Subtitle("über den öffentlichen Upload-Dienst").Value("immer an"),
			ui.Row("USB-Stick").Subtitle("erscheint unter Fotos, sobald eingesteckt").Value("automatisch"),
			ui.Row("Kamera per USB").Subtitle("Aufnahmen landen im Eingang").Value(camera),
		),
	).Gap(u(18))
}

// nasSettings richtet die Netzwerkfreigabe ein: Adresse, Benutzer und
// Kennwort eintragen, anmelden, eine der gefundenen Freigaben antippen.
// Gespeichert wird erst mit der Wahl der Freigabe, also erst, wenn die
// Anmeldung geklappt hat.
func (a *App) nasSettings(ctx *gift.Context, st *states, s device.Settings, save func(func(*device.Settings))) gift.View {
	cfg := s.NAS.Normalized()
	hostEd := ui.Editor(ctx, "nashost", cfg.Host)
	userEd := ui.Editor(ctx, "nasuser", cfg.User)
	passEd := ui.Editor(ctx, "naspass", "")
	rev := ctx.State("rev", 0)
	searched := ctx.State("searched", false)

	// Ist ein NAS eingerichtet, zeigt die Box, ob es gerade erreichbar ist.
	status := xgift.UseResource[nas.Listing](ctx, "status")
	if cfg.Configured() {
		status.LoadKeyed([2]any{cfg, ctx.Read(rev)}, func() (nas.Listing, error) {
			c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			return a.dev.NAS.Browse(a.dev.Subject(), c, ".")
		})
	}

	typed := func() nas.Config {
		return nas.Config{Host: hostEd.Text(), User: userEd.Text(), Password: passEd.Text()}.Normalized()
	}

	shares := xgift.UseResource[[]string](ctx, "shares")
	search := func() {
		want := typed()
		searched.Set(true)
		shares.Load(func() ([]string, error) {
			c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			return a.dev.NAS.Shares(a.dev.Subject(), c, want)
		})
	}

	choose := func(share string) {
		want := typed()
		save(func(s *device.Settings) {
			pw := want.Password
			if old := s.NAS.Normalized(); pw == "" && old.Host == want.Host && old.User == want.User {
				pw = old.Password
			}

			s.NAS = nas.Config{Host: want.Host, User: want.User, Password: pw, Share: share}
		})

		passEd.SetText("")
		searched.Set(false)
		rev.Set(rev.Get() + 1)
		a.show("NAS eingerichtet: " + share + " auf " + want.Host)
	}

	field := func(ed *ui.TextEditor, placeholder string) gift.View {
		return ui.TextField(ed).Placeholder(placeholder).FontSize(u(16)).Frame(u(380), u(44))
	}

	passHint := "Kennwort"
	if cfg.Password != "" {
		passHint = "gespeichert – leer lassen"
	}

	var rows []gift.View
	if cfg.Configured() {
		state := "wird geprüft …"
		switch {
		case status.Err() != nil:
			state = status.Err().Error()
		case status.Loaded():
			l := status.Value()
			state = fmt.Sprintf("erreichbar · %d Ordner · %d Bilder oben", len(l.Folders), len(l.Images))
		}

		rows = append(rows,
			ui.Row(cfg.Title()).Subtitle(state).Value("✓"),
			ui.Row("Fotos durchsuchen").Chevron(outline.AngleRight).OnTap(func() { a.openLibrary(photo.ScopeAll, sourceNAS) }),
		)
	}

	rows = append(rows,
		ui.Row("Adresse").Accessory(field(hostEd, "diskstation.local oder 192.168.178.20")),
		ui.Row("Benutzer").Accessory(field(userEd, "Benutzer auf dem NAS")),
		ui.Row("Kennwort").Accessory(field(passEd, passHint)),
	)

	var found gift.View = ui.Box().Frame(1, 1)
	switch {
	case !ctx.Read(searched):
	case shares.Loading() || !shares.Loaded():
		found = muted("Anmelden …", 15).PaddingInsets(geom.Insets{Left: u(16)})
	case shares.Err() != nil:
		found = body(shares.Err().Error(), 15).MaxLines(3).Foreground(red).PaddingInsets(geom.Insets{Left: u(16)})
	case len(shares.Value()) == 0:
		found = muted("Angemeldet, aber dieser Benutzer sieht keine Freigabe.", 15).PaddingInsets(geom.Insets{Left: u(16)})
	default:
		chips := []gift.View{muted("Freigabe wählen:", 15)}
		for _, name := range shares.Value() {
			chips = append(chips, xgift.Chip(name, name == cfg.Share, u(15), blue, func() { choose(name) }))
		}

		found = ui.HScroll(chips...).Gap(u(8)).PaddingInsets(geom.Insets{Left: u(16), Right: u(16)}).MinHeight(u(48))
	}

	actions := []gift.View{primary("Anmelden und Freigaben suchen", search)}
	if cfg.Host != "" {
		actions = append(actions, secondary("NAS entfernen", func() {
			save(func(s *device.Settings) { s.NAS = nas.Config{} })
			hostEd.SetText("")
			userEd.SetText("")
			passEd.SetText("")
			searched.Set(false)
			a.show("NAS entfernt. Übernommene Fotos bleiben auf der Box.")
		}))
	}

	return ui.VStack(
		section("NAS IM HEIMNETZ (SMB)", rows...),
		ui.HStack(actions...).Gap(u(12)),
		found,
		muted("Bei einer Synology DiskStation liegen die Fotos meist in der Freigabe „photo“ oder „home“. Am besten legst du einen eigenen Benutzer an, der die Fotos nur lesen darf: Das Kennwort liegt auf der Speicherkarte der Box.", 13).MaxLines(4).PaddingInsets(geom.Insets{Left: u(16)}),
	).Gap(u(14))
}

// --- Kiosk ------------------------------------------------------------------

func (a *App) kioskSettings(ctx *gift.Context, st *states, s device.Settings, save func(func(*device.Settings))) gift.View {
	titleEd := ui.Editor(ctx, "eventtitle", s.EventTitle)
	pinEd := ui.Editor(ctx, "pin", "")

	accents := []gift.View{}
	for _, c := range []string{"#E9B949", "#E07A8F", "#7FB7A4", "#9DB4E8"} {
		ring := ui.Border{Width: 1, Color: ui.ColorSeparator}
		if strings.EqualFold(s.Accent, c) {
			ring = ui.Border{Width: u(3), Color: blue}
		}

		face := ui.ButtonStyle{Background: parseHex(c, amber), Border: ring, CornerRadius: u(20)}
		accents = append(accents, ui.Button(ui.Box().Frame(u(22), u(22)), func() { save(func(s *device.Settings) { s.Accent = c }) }).
			Style(face).HoverStyle(face).PressedStyle(face).Frame(u(44), u(44)).Label("Farbe "+c))
	}

	layoutChips := []gift.View{}
	defaultChips := []gift.View{}
	for _, t := range printing.Templates() {
		on := slices.Contains(s.KioskLayouts, t.ID)
		layoutChips = append(layoutChips, xgift.Chip(t.Name, on, u(14), blue, func() {
			save(func(s *device.Settings) {
				if slices.Contains(s.KioskLayouts, t.ID) {
					if len(s.KioskLayouts) > 1 {
						s.KioskLayouts = slices.DeleteFunc(slices.Clone(s.KioskLayouts), func(x printing.TemplateID) bool { return x == t.ID })
					}
				} else {
					s.KioskLayouts = append(slices.Clone(s.KioskLayouts), t.ID)
				}
			})
		}))
		defaultChips = append(defaultChips, xgift.Chip(t.Name, s.KioskDefault == t.ID, u(14), blue, func() {
			save(func(s *device.Settings) { s.KioskDefault = t.ID })
		}))
	}

	pinState := "nicht festgelegt"
	if s.Pin.Configured() {
		pinState = "festgelegt"
	}

	return ui.VStack(
		ui.HStack(
			xgift.IconTile(outline.WandMagicSparkles, pink, u(56)),
			muted("Für Feiern: Gäste drucken selbst, deine Mediathek bleibt verborgen. Aktiv bis zum nächsten Neustart.", 14).MaxLines(3).Flex(1),
			primary("Kiosk starten …", func() {
				if t := strings.TrimSpace(titleEd.Text()); t != s.EventTitle {
					save(func(s *device.Settings) { s.EventTitle = t })
				}

				a.openSheet(SheetKioskStart)
			}),
		).Gap(u(16)).Align(geom.Center).Padding(u(18)).Background(ui.ColorSurface).CornerRadius(u(14)),
		section("VERANSTALTUNG",
			ui.Row("Titel").Accessory(ui.TextField(titleEd).Placeholder("z. B. Hochzeit Anna & Ben").FontSize(u(16)).Frame(u(360), u(44)).
				OnSubmit(func(v string) { save(func(s *device.Settings) { s.EventTitle = strings.TrimSpace(v) }) })),
			ui.Row("Eventfarbe").Accessory(ui.HStack(accents...).Gap(u(6))),
		),
		section("DRUCKEN IM KIOSK",
			ui.Row("Handy-Uploads sofort drucken").Accessory(ui.Toggle(s.PrintUploads, func(v bool) { save(func(s *device.Settings) { s.PrintUploads = v }) })),
			ui.Row("Kamerabilder sofort drucken").Accessory(ui.Toggle(s.PrintCamera, func(v bool) { save(func(s *device.Settings) { s.PrintCamera = v }) })),
			chipRow("Layouts für Gäste", layoutChips),
			chipRow("Vorgabe ohne Wahl", defaultChips),
			ui.Row("Höchstens je Foto").Accessory(xgift.Stepper(s.MaxCopies, 1, 5, u(16), func(v int) { save(func(s *device.Settings) { s.MaxCopies = v }) })),
		),
		section("ZUGANG",
			ui.Row("Betreuer-PIN").Value(pinState),
			ui.Row("Neue PIN").Accessory(ui.HStack(
				ui.TextField(pinEd).Placeholder("6 Ziffern").FontSize(u(16)).Frame(u(180), u(44)),
				secondary("Festlegen", func() {
					if !a.fail(a.dev.Device.SetPin(a.dev.Subject(), strings.TrimSpace(pinEd.Text()))) {
						pinEd.SetText("")
						save(func(*device.Settings) {})
						a.show("PIN festgelegt.")
					}
				}),
			).Gap(u(10)).Align(geom.Center)),
		),
		muted("Im Kiosk sind Einstellungen und Mediathek gesperrt. Zurück zum Heimbetrieb: Box aus- und wieder einschalten – oder fünfmal schnell auf den QR-Code tippen und die PIN eingeben.", 13).MaxLines(3).PaddingInsets(geom.Insets{Left: u(16)}),
	).Gap(u(18))
}

// --- Speicher & USB ---------------------------------------------------------

func (a *App) storageSettings(ctx *gift.Context, st *states, s device.Settings) gift.View {
	rev := ctx.State("rev", 0)
	usage := xgift.UseResource[photo.Usage](ctx, "usage")
	usage.LoadKeyed([2]int{ctx.Read(rev), ctx.Read(st.photos)}, func() (photo.Usage, error) { return a.dev.Photos.InspectStorage(a.dev.Subject()) })
	us := usage.Value()

	events := slices.Clone(s.Events)
	slices.Reverse(events)

	rows := []gift.View{}
	for _, e := range events {
		rows = append(rows, ui.Row(e.Title).Subtitle(e.StartedAt.Local().Format(" 02.01.2006 15:04")).
			Chevron(outline.AngleRight).OnTap(func() {
			a.exportEvent = e.ID
			a.exportAll = false
			a.openSheet(SheetExport)
		}).Key(string(e.ID)))
	}

	if len(rows) == 0 {
		rows = append(rows, ui.Row("Noch keine Feier").Subtitle("Fotos aus dem Kiosk erscheinen hier je Feier."))
	}

	return ui.VStack(
		section("SPEICHERKARTE",
			ui.Row("Belegt durch Fotos").Value(fmt.Sprintf("%s · %d Dateien", gib(us.Photos), us.Files)),
			ui.Row("Frei").Value(gib(us.Free)),
			ui.Row("Gesamt").Value(gib(us.Total)),
		),
		section("AUF USB-STICK KOPIEREN",
			append(rows, ui.Row("Alle Fotos").Chevron(outline.AngleRight).OnTap(func() {
				a.exportEvent = ""
				a.exportAll = true
				a.openSheet(SheetExport)
			}))...,
		),
		muted("Stick einstecken, Feier wählen, kopieren. Die Originale landen in einem Ordner mit dem Namen der Feier.", 13).MaxLines(2).PaddingInsets(geom.Insets{Left: u(16)}),
	).Gap(u(18))
}

func gib(b int64) string {
	return strings.Replace(fmt.Sprintf("%.1f GB", float64(b)/1e9), ".", ",", 1)
}

// --- Info -------------------------------------------------------------------

func (a *App) infoSettings(ctx *gift.Context, st *states) gift.View {
	version := "unbekannt"
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, kv := range bi.Settings {
			if kv.Key == "vcs.revision" && len(kv.Value) >= 7 {
				version = kv.Value[:7]
			}
		}
	}

	rows := []gift.View{
		ui.Row("Version").Value(version),
		ui.Row("Daten").Subtitle(a.dev.Options.DataDir),
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, ad := range addrs {
			if ip, ok := ad.(*net.IPNet); ok && !ip.IP.IsLoopback() && ip.IP.To4() != nil {
				rows = append(rows, ui.Row("IP-Adresse").Value(ip.IP.String()))
			}
		}
	}

	return section("GERÄT", rows...)
}

// chipRow ist eine Zeile mit Beschriftung und Chips darunter. Nebeneinander
// passen mehrere Chips auf kleinen Panels nicht in eine Listenzeile; so
// laufen sie seitlich weiter und lassen sich wischen.
func chipRow(label string, chips []gift.View) gift.View {
	return ui.VStack(
		body(label, 16),
		ui.HScroll(chips...).Gap(u(6)),
	).Gap(u(8)).PaddingInsets(geom.Insets{Left: u(16), Right: u(16), Top: u(10), Bottom: u(10)})
}
