package uidevice

import (
	"context"
	"fmt"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
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

// homeShell ist der Rahmen des Heimbetriebs: Statusleiste und Bildschirm.
func (a *App) homeShell(ctx *gift.Context, st *states) gift.View {
	screen := ctx.Read(st.screen)

	var content gift.View
	switch screen {
	case ScreenLibrary:
		content = gift.Component("library", func(ctx *gift.Context) gift.View { return a.library(ctx, st) })
	case ScreenStudio:
		content = gift.Component("studio", func(ctx *gift.Context) gift.View { return a.studioScreen(ctx, st) })
	case ScreenJobs:
		content = gift.Component("jobs", func(ctx *gift.Context) gift.View { return a.jobsScreen(ctx, st) })
	case ScreenSettings:
		content = gift.Component("settings", func(ctx *gift.Context) gift.View { return a.settingsScreen(ctx, st) })
	default:
		content = gift.Component("homescreen", func(ctx *gift.Context) gift.View { return a.homeScreen(ctx, st) })
	}

	bg := ui.ColorBackground
	if screen == ScreenHome {
		bg = wallpaper
	}

	return ui.VStack(
		gift.Component("statusbar", func(ctx *gift.Context) gift.View { return a.statusBar(ctx, st, false, "") }),
		grow(content),
	).Background(bg).Flex(1)
}

// status sind die Angaben der Statusleiste.
type status struct {
	wifi    wifi.Status
	paper   int
	relay   relay.State
	printer string
}

// statusBar zeigt Uhrzeit, Papier, Upload-Dienst und Funknetz.
func (a *App) statusBar(ctx *gift.Context, st *states, dark bool, pill string) gift.View {
	tick := ctx.Read(st.tick)
	res := xgift.UseResource[status](ctx, "status")
	res.LoadKeyed(tick/5, func() (status, error) { return a.loadStatus(), nil })

	s := res.Value()
	now := time.Now()

	fg := ui.ColorLabel
	if dark {
		fg = white
	}

	cloud := "Upload"
	if s.relay != relay.StateReady {
		cloud = "Kein Upload"
	}

	net := "Kein WLAN"
	if s.wifi.Connected {
		net = s.wifi.SSID
	}

	items := []gift.View{
		ui.Text(now.Format("15:04")).FontSize(u(13)).Font(boldFont).Foreground(fg),
		ui.Text(weekday(now) + now.Format(" 2. ") + month(now)).FontSize(u(13)).Foreground(ui.ColorSecondaryLabel),
		fill(),
	}

	if pill != "" {
		items = append(items, ui.Text(pill).FontSize(u(12)).Font(boldFont).Foreground(ink).
			Background(ui.ColorAccent).CornerRadius(u(10)).
			PaddingInsets(geom.Insets{Left: u(10), Right: u(10), Top: u(2), Bottom: u(2)}))
	}

	for _, it := range []struct {
		sym   ui.Symbol
		label string
	}{
		{outline.Printer, fmt.Sprintf("%d Blatt", s.paper)},
		{outline.CloudArrowUp, cloud},
		{outline.Globe, net},
	} {
		items = append(items, ui.HStack(
			ui.Icon(it.sym).Size(u(16)).Foreground(fg),
			ui.Text(it.label).FontSize(u(13)).Font(boldFont).Foreground(fg),
		).Gap(u(6)).Align(geom.Center))
	}

	return ui.HStack(items...).Gap(u(18)).Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(28), Right: u(28)}).
		MinHeight(u(32))
}

// loadStatus sammelt die Angaben der Statusleiste. Sie laufen als System:
// Auch ein Gast soll sehen, ob Papier da ist, und die Angaben verraten
// nichts Privates.
func (a *App) loadStatus() status {
	var s status

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if w, err := a.dev.WiFi.Current(sys(), ctx); err == nil {
		s.wifi = w
	}

	if set, err := a.dev.Device.LoadSettings(sys()); err == nil {
		s.paper = set.PaperLeft
	}

	if addr, err := a.dev.Relay.UploadAddress(sys(), true); err == nil {
		s.relay = addr.State
	}

	return s
}

// homeData ist alles, was der Home-Bildschirm zeigt.
type homeData struct {
	inbox    []photo.Location
	unseen   int
	settings device.Settings
	printer  printing.PrinterStatus
	address  relay.Address
}

// homeScreen ist der Home-Bildschirm mit Widgets und App-Symbolen.
func (a *App) homeScreen(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)

	res := xgift.UseResource[homeData](ctx, "home")
	res.LoadKeyed([2]int{tick / 3, ctx.Read(st.photos)}, func() (homeData, error) { return a.loadHome() })
	d := res.Value()

	left := xgift.Fill(ui.VStack(
		a.inboxWidget(st, d),
		xgift.HStretch(xgift.Fill(a.printerWidget(st, d)).Flex(1), xgift.Fill(a.phoneWidget(d)).Flex(1)).Gap(u(20)).Flex(1),
	).Gap(u(20))).Width(u(800))

	right := xgift.Fill(ui.VStack(
		a.appIcons(st),
		xgift.Fill(a.kioskWidget(st, d)).Flex(1),
	).Gap(u(20))).Flex(1)

	return ui.VStack(xgift.HStretch(left, right).Gap(u(24)).Flex(1)).
		PaddingInsets(geom.Insets{Top: u(16), Left: u(40), Right: u(40), Bottom: u(32)})
}

func (a *App) loadHome() (homeData, error) {
	var d homeData
	subject := a.dev.Subject()

	inbox, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: photo.ScopeInbox})
	if err != nil {
		return d, err
	}

	var ids []photo.ID
	for i, p := range inbox {
		if p.Unseen {
			d.unseen++
		}

		if i < 5 {
			ids = append(ids, p.ID)
		}
	}

	if d.inbox, err = a.dev.Photos.Locate(subject, ids...); err != nil {
		return d, err
	}

	if d.settings, err = a.dev.Device.LoadSettings(subject); err != nil {
		return d, err
	}

	d.printer, _ = a.dev.Printing.Diagnose(subject)
	d.address, _ = a.dev.Relay.UploadAddress(subject, true)

	return d, nil
}

func (a *App) inboxWidget(st *states, d homeData) gift.View {
	thumbs := []gift.View{}
	for _, loc := range d.inbox {
		thumbs = append(thumbs, thumb(loc.Path, u(138)))
	}

	if len(thumbs) == 0 {
		thumbs = append(thumbs, muted("Noch nichts angekommen. Scanne den QR-Code mit dem Handy, um Fotos zu senden.", 16).Flex(1).MinHeight(u(138)))
	}

	badge := gift.View(ui.Box().Frame(1, 1))
	if d.unseen > 0 {
		badge = ui.Badge(fmt.Sprintf("%d neu", d.unseen)).Color(blue)
	}

	return card(
		ui.HStack(
			xgift.IconTile(outline.Inbox, ui.RGB(0xE8, 0x59, 0x0C), u(34)),
			title("Eingang", 21),
			badge,
			fill(),
			link("Alle ansehen", func() { a.openLibrary(photo.ScopeInbox, "") }),
		).Gap(u(12)).Align(geom.Center),
		ui.HStack(thumbs...).Gap(u(12)),
		ui.HStack(
			muted("Fotos von Handy und Kamera warten hier, bis du Format und Design wählst.", 15).Flex(1),
			primary("Auswählen und drucken", func() { a.openLibrary(photo.ScopeInbox, "") }),
		).Gap(u(16)).Align(geom.Center),
	)
}

func (a *App) printerWidget(st *states, d homeData) gift.View {
	state, color := "Bereit", greenText
	switch {
	case d.settings.Printer.TestMode():
		state, color = "Testbetrieb", orange
	case !d.printer.OK():
		state, color = "Gestört", red
	}

	capacity := max(d.settings.PaperCapacity, 1)
	frac := float64(d.settings.PaperLeft) / float64(capacity)

	detail := "Citizen CZ-01 · 10 × 15 cm"
	if p := d.printer.Problem(); p != "" && !d.settings.Printer.TestMode() {
		detail = p
	}

	return card(
		ui.HStack(
			xgift.IconTile(outline.Printer, grey, u(34)),
			title("Drucker", 21),
			fill(),
			ui.Text(state).FontSize(u(15)).Font(boldFont).Foreground(color),
		).Gap(u(12)).Align(geom.Center),
		ui.HStack(
			ui.Text(fmt.Sprint(d.settings.PaperLeft)).FontSize(u(56)).Font(boldFont),
			body(fmt.Sprintf("von %d Blatt übrig", capacity), 17),
		).Gap(u(8)).AlignBaseline(),
		ui.ProgressBar(frac).Tint(green).Frame(geom.Unbounded(), u(10)),
		muted(detail, 15).MaxLines(2),
		fill(),
		link("Aufträge ansehen", func() { a.st.screen.Set(ScreenJobs) }),
	).Flex(1)
}

func (a *App) phoneWidget(d homeData) gift.View {
	var code gift.View
	if d.address.URL != "" {
		code = xgift.QRCode(d.address.URL, u(150))
	} else {
		code = ui.VStack(muted(orDash(d.address.Problem), 14).MaxLines(4)).Frame(u(150), u(150)).
			Align(geom.Center).Background(ui.ColorBackground).CornerRadius(u(12)).Padding(u(10))
	}

	return card(
		ui.HStack(
			xgift.IconTile(outline.MobilePhone, green, u(34)),
			title("Vom Handy senden", 21),
		).Gap(u(12)).Align(geom.Center),
		ui.HStack(
			code,
			muted("Kamera öffnen, Code scannen, Fotos wählen. Sie landen im Eingang – gedruckt wird erst, wenn du hier auswählst.", 15).MaxLines(6).Flex(1),
		).Gap(u(18)).Align(geom.Center).Flex(1),
	).Flex(1)
}

func (a *App) appIcons(st *states) gift.View {
	type app struct {
		label string
		sym   ui.Symbol
		face  ui.Color
		fn    func()
	}

	apps := []app{
		{"Fotos", outline.Image, ui.RGB(0xE8, 0x59, 0x0C), func() { a.openLibrary(photo.ScopeAll, "") }},
		{"Drucken", outline.Printer, blue, func() {
			if len(a.studio) > 0 {
				st.screen.Set(ScreenStudio)
			} else {
				a.openLibrary(photo.ScopeInbox, "")
				a.show("Wähle zuerst Fotos aus.")
			}
		}},
		{"Aufträge", outline.List, green, func() { st.screen.Set(ScreenJobs) }},
		{"USB-Stick", outline.ArchiveArrowDown, grey, func() { a.openLibrary(photo.ScopeAll, sourceUSB) }},
		{"Einstellungen", outline.Cog, ui.RGB(0x5E, 0x5E, 0x63), func() { st.screen.Set(ScreenSettings) }},
	}

	cells := make([]gift.View, 0, len(apps))
	for _, ap := range apps {
		face := ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}
		cells = append(cells, ui.Button(ui.VStack(
			xgift.IconTile(ap.sym, ap.face, u(84)),
			ui.Text(ap.label).FontSize(u(14)),
		).Gap(u(8)).Align(geom.Center), ap.fn).
			Style(face).HoverStyle(face).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.06), CornerRadius: u(16)}).
			Label(ap.label))
	}

	return xgift.Grid(3, u(18), cells...)
}

func (a *App) kioskWidget(st *states, d homeData) gift.View {
	name := d.settings.EventTitle
	if name == "" {
		name = "Nächste Feier"
	}

	return ui.VStack(
		ui.HStack(
			ui.Icon(outline.WandMagicSparkles).Size(u(20)).Foreground(amber),
			ui.Text("KIOSK-MODUS").FontSize(u(13)).Font(boldFont).Foreground(amber),
		).Gap(u(10)).Align(geom.Center),
		title(name, 24).Foreground(white).MaxLines(2),
		ui.Text("Gäste drucken selbst, deine Mediathek bleibt verborgen. Aktiv bis zum nächsten Neustart.").
			FontSize(u(15)).Foreground(ui.RGB(0xC7, 0xC7, 0xCC)).MaxLines(4),
		fill(),
		filled("Kiosk starten", amber, ink, func() { a.openSheet(SheetKioskStart) }),
	).Gap(u(12)).Padding(u(24)).Background(ui.RGB(0x16, 0x16, 0x1A)).CornerRadius(u(26)).Flex(1)
}

// thumb ist ein quadratisches Vorschaubild eines Originals.
func thumb(path string, edge float32) ui.ImageView {
	return ui.Image(asset.File(path)).
		Fit(ui.FitCover).
		Frame(edge, edge).
		CornerRadius(edge / 10).
		Clip(true).
		Placeholder(ui.Fade(ui.ColorLabel, 0.08))
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}

	return s
}

func weekday(t time.Time) string {
	return [...]string{"So.", "Mo.", "Di.", "Mi.", "Do.", "Fr.", "Sa."}[t.Weekday()]
}

func month(t time.Time) string {
	return [...]string{"Jan.", "Feb.", "März", "Apr.", "Mai", "Juni", "Juli", "Aug.", "Sep.", "Okt.", "Nov.", "Dez."}[t.Month()-1]
}
