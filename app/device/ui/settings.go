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
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/app/wifi"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// Abschnitte der Einstellungen.
const (
	sectionWifi = iota
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

	res := xgift.UseResource[device.Settings](ctx, "settings")
	res.LoadKeyed(ctx.Read(rev), func() (device.Settings, error) { return a.dev.Device.LoadSettings(a.dev.Subject()) })
	s := res.Value()

	save := func(fn func(*device.Settings)) {
		if _, err := a.dev.Device.SaveSettings(a.dev.Subject(), fn); !a.fail(err) {
			rev.Set(rev.Get() + 1)
		}
	}

	rows := []gift.View{}
	for _, sec := range sections {
		fg, face := ui.ColorLabel, ui.ColorSurface
		if sec.id == current {
			fg, face = white, blue
		}

		style := ui.ButtonStyle{Background: face, Border: noBorder, CornerRadius: u(10)}
		rows = append(rows, ui.Button(ui.HStack(
			xgift.IconTile(sec.sym, sec.face, u(30)),
			ui.Text(sec.label).FontSize(u(16)).Foreground(fg).Flex(1),
			ui.Icon(outline.AngleRight).Size(u(16)).Foreground(fg),
		).Gap(u(12)).Align(geom.Center), func() { st.settings.Set(sec.id) }).
			Style(style).HoverStyle(style).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(blue, 0.2), CornerRadius: u(10)}).
			PaddingInsets(geom.Insets{Left: u(12), Right: u(12)}).
			MinHeight(u(46)))
	}

	sidebar := ui.VStack(
		link("‹ Home", func() { st.screen.Set(ScreenHome) }),
		title("Einstellungen", 30),
		ui.HStack(
			xgift.IconTile(outline.Printer, ink, u(48)),
			ui.VStack(title("eventprint", 17), muted("Heimdrucker · Citizen CZ-01", 13)).Gap(u(2)),
		).Gap(u(14)).Align(geom.Center).Padding(u(12)).Background(ui.ColorSurface).CornerRadius(u(12)),
		ui.VStack(rows...).Gap(u(4)),
	).Gap(u(12)).Padding(u(20)).Background(ui.ColorBackground)

	var detail gift.View
	key := fmt.Sprint("detail", current)
	switch current {
	case sectionWifi:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.wifiSettings(ctx, st) })
	case sectionPrinter:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.printerSettings(ctx, st, s, save) })
	case sectionUpload:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.uploadSettings(ctx, st, s, save) })
	case sectionSources:
		detail = gift.Component(key, func(ctx *gift.Context) gift.View { return a.sourceSettings(ctx, st) })
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

	return xgift.HStretch(
		xgift.Fill(ui.VScroll(sidebar)).Width(u(400)),
		xgift.VHairline(),
		grow(ui.VStack(
			title(name, 17).Align(ui.AlignCenter).PaddingInsets(geom.Insets{Top: u(16), Bottom: u(8)}),
			ui.VScroll(ui.VStack(detail, ui.Box().Frame(1, ui.OnScreenKeyboardHeight())).MaxWidth(u(700)).PaddingInsets(geom.Insets{Left: u(24), Right: u(24), Bottom: u(24)})).Flex(1),
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
		status = res.Err().Error()
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
		chips = append(chips, xgift.Chip(q, s.Printer.Queue == q, u(15), blue, func() {
			save(func(s *device.Settings) { s.Printer.Queue = q })
		}))
	}

	speed := 0
	if s.Printer.Speed() == printing.SpeedNormal {
		speed = 1
	}

	return ui.VStack(
		section("WARTESCHLANGE",
			ui.Row("").Accessory(ui.HStack(chips...).Gap(u(8))),
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
				}).FontSize(u(14)).Frame(u(220), u(36))),
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
	tokenEd := ui.Editor(ctx, "relaytoken", s.RelayToken)

	addr := xgift.UseResource[relay.Address](ctx, "addr")
	addr.LoadKeyed(tick/2, func() (relay.Address, error) { return a.dev.Relay.UploadAddress(a.dev.Subject(), true) })

	var code gift.View = muted(orDash(addr.Value().Problem), 15).MaxLines(3)
	if u := addr.Value().URL; u != "" {
		code = ui.HStack(xgift.QRCode(u, 140*scale), muted(u, 13).MaxLines(4).Flex(1)).Gap(16 * scale).Align(geom.Center)
	}

	return ui.VStack(
		section("ZUSTAND", ui.Row("").Accessory(code)),
		section("UPLOAD-DIENST",
			ui.Row("Adresse").Accessory(ui.TextField(urlEd).Placeholder("https://upload.example.de").FontSize(u(16)).Frame(u(380), u(44))),
			ui.Row("Token").Accessory(ui.TextField(tokenEd).Placeholder("Zugangstoken").FontSize(u(16)).Frame(u(380), u(44))),
		),
		muted("Das Token stammt aus dem Upload-Dienst (Rolle „Fotobox-Relay“). Am bequemsten trägt man beides in /etc/default/eventprint ein.", 13).MaxLines(3).PaddingInsets(geom.Insets{Left: u(16)}),
		primary("Speichern", func() {
			url, token := strings.TrimSpace(urlEd.Text()), strings.TrimSpace(tokenEd.Text())
			save(func(s *device.Settings) { s.RelayURL, s.RelayToken = url, token })
			a.show("Gespeichert. Die Verbindung wird neu aufgebaut.")
		}),
	).Gap(u(18))
}

// --- Konten & Quellen -------------------------------------------------------

func (a *App) sourceSettings(ctx *gift.Context, st *states) gift.View {
	camera := "abgeschaltet"
	if a.dev.Camera != nil {
		cs := a.dev.Camera.Status()
		camera = "keine Kamera"
		if cs.Model != "" {
			camera = cs.Model
		}
	}

	return ui.VStack(
		section("QUELLEN",
			ui.Row("Handy-Upload per QR").Subtitle("über den öffentlichen Upload-Dienst").Value("immer an"),
			ui.Row("USB-Stick").Subtitle("erscheint unter Fotos, sobald eingesteckt").Value("automatisch"),
			ui.Row("Kamera per USB").Subtitle("Aufnahmen landen im Eingang").Value(camera),
			ui.Row("Netzwerkfreigabe (SMB/NAS)").Subtitle("Fotos vom NAS im Heimnetz").Value("bald").Disabled(true),
		),
	).Gap(u(18))
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
		accents = append(accents, ui.Button(ui.Box().Frame(u(26), u(26)), func() { save(func(s *device.Settings) { s.Accent = c }) }).
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
			ui.Row("Layouts für Gäste").Accessory(ui.HStack(layoutChips...).Gap(u(6))),
			ui.Row("Vorgabe ohne Wahl").Accessory(ui.HStack(defaultChips...).Gap(u(6))),
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
		rows = append(rows, ui.Row(e.Title).Subtitle(e.StartedAt.Local().Format("Mon 02.01.2006 15:04")).
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
		ui.Row("Daten").Value(a.dev.Options.DataDir),
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
