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

// homeShell ist der Rahmen des Heimbetriebs: Hintergrundbild, Statusleiste
// und Bildschirm.
//
// Das Hintergrundbild liegt unter allem, auch unter der Statusleiste; der
// Home-Bildschirm ist durchsichtig und zeigt es, die anderen Bildschirme
// decken es mit ihrer eigenen Fläche ab. So steht es beim Wechsel still,
// während die Seiten darüber gleiten.
func (a *App) homeShell(ctx *gift.Context, st *states) gift.View {
	screen := ctx.Read(st.screen)
	if screen != a.nav.current {
		a.nav.previous, a.nav.current = a.nav.current, screen
	}

	return ui.ZStack(
		xgift.Fill(gift.Component("wallpaper", func(ctx *gift.Context) gift.View { return a.wallpaper(ctx, st) })),
		xgift.Fill(ui.VStack(
			gift.Component("statusbar", func(ctx *gift.Context) gift.View { return a.statusBar(ctx, st, false, "") }),
			grow(xgift.Pages(a.page(st, a.nav.current), a.page(st, a.nav.previous))),
		)),
	).Flex(1)
}

// wallpaper ist das Hintergrundbild des Heimbetriebs: das neueste Foto,
// weichgezeichnet; siehe wallpaper.go.
func (a *App) wallpaper(ctx *gift.Context, st *states) gift.View {
	tone := wallLight
	if a.dark {
		tone = wallDark
	}

	res := xgift.UseResource[*xgift.MemorySource](ctx, "wall")
	res.LoadKeyed([3]any{ctx.Read(st.tick) / 10, ctx.Read(st.photos), tone}, func() (*xgift.MemorySource, error) {
		return wallpaperOf(a.newestPhoto(), tone), nil
	})

	if res.Value() == nil {
		return ui.Box().Background(wallpaper)
	}

	return ui.Image(res.Value()).Fit(ui.FitCover).Placeholder(wallpaper)
}

// newestPhoto ist der Pfad des neuesten Fotos: im Eingang, sonst in der
// Mediathek, sonst keiner.
func (a *App) newestPhoto() string {
	subject := a.dev.Subject()
	for _, scope := range []photo.Scope{photo.ScopeInbox, photo.ScopeAll} {
		all, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: scope})
		if err != nil || len(all) == 0 {
			continue
		}

		if locs, err := a.dev.Photos.Locate(subject, all[0].ID); err == nil && len(locs) > 0 {
			return locs[0].Path
		}
	}

	return ""
}

// depth ordnet die Bildschirme für die Bewegung beim Wechsel: Tiefer
// liegende schieben sich von rechts über den Home-Bildschirm, zurück geht es
// nach rechts hinaus.
func (s Screen) depth() int {
	switch s {
	case ScreenHome:
		return 0
	case ScreenStudio:
		return 2
	default:
		return 1
	}
}

// page baut einen Bildschirm als Seite mit eigenem, deckendem Hintergrund:
// Während eines Wechsels liegen zwei Seiten übereinander. Nur der
// Home-Bildschirm ist durchsichtig; unter ihm liegt das Hintergrundbild, und
// jede andere Seite gleitet deckend über ihn.
func (a *App) page(st *states, s Screen) xgift.Page {
	var content gift.View
	bg := ui.ColorBackground
	switch s {
	case ScreenLibrary:
		content = gift.Component("library", func(ctx *gift.Context) gift.View { return a.library(ctx, st) })
	case ScreenStudio:
		content = gift.Component("studio", func(ctx *gift.Context) gift.View { return a.studioScreen(ctx, st) })
		bg = ui.ColorClear
	case ScreenJobs:
		content = gift.Component("jobs", func(ctx *gift.Context) gift.View { return a.jobsScreen(ctx, st) })
	case ScreenSettings:
		content = gift.Component("settings", func(ctx *gift.Context) gift.View { return a.settingsScreen(ctx, st) })
	default:
		content = gift.Component("homescreen", func(ctx *gift.Context) gift.View { return a.homeScreen(ctx, st) })
		bg = ui.ColorClear
	}

	return xgift.Page{
		Key:   fmt.Sprint("screen", int(s)),
		Depth: s.depth(),
		View:  ui.VStack(grow(content)).Background(bg),
	}
}

// status sind die Angaben der Statusleiste.
type status struct {
	wifi    wifi.Status
	paper   int
	relay   relay.State
	printer string
}

// statusBar zeigt Uhrzeit, Papier, Upload-Dienst und Funknetz. Im
// Heimbetrieb stehen die Angaben in Glaskapseln über dem Hintergrundbild, im
// Kiosk (dark) schlicht auf dem dunklen Grund.
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

	cloud, cloudOK := "Upload", s.relay == relay.StateReady
	if !cloudOK {
		cloud = "Kein Upload"
	}

	net := "Kein WLAN"
	if s.wifi.Connected {
		net = s.wifi.SSID
	}

	clock := ui.HStack(
		ui.Text(now.Format("15:04")).FontSize(u(13)).Font(boldFont).Foreground(fg),
		ui.Text(weekday(now) + now.Format(" 2. ") + month(now)).FontSize(u(13)).Foreground(ui.ColorSecondaryLabel),
	).Gap(u(7)).Align(geom.Center)

	type item struct {
		lead  gift.View
		label string
	}

	dot := ui.Box().Frame(u(8), u(8)).CornerRadius(u(4)).Background(green)
	if !cloudOK {
		dot = dot.Background(orange)
	}

	items := []item{
		{ui.Icon(outline.Printer).Size(u(16)).Foreground(fg), fmt.Sprintf("%d Blatt", s.paper)},
		{dot, cloud},
		{ui.Icon(outline.Globe).Size(u(16)).Foreground(fg), net},
	}

	if dark {
		row := []gift.View{clock, fill()}
		if pill != "" {
			row = append(row, ui.Text(pill).FontSize(u(12)).Font(boldFont).Foreground(ink).
				Background(ui.ColorAccent).CornerRadius(u(10)).
				PaddingInsets(geom.Insets{Left: u(10), Right: u(10), Top: u(2), Bottom: u(2)}))
		}

		for _, it := range items {
			row = append(row, ui.HStack(it.lead, ui.Text(it.label).FontSize(u(13)).Font(boldFont).Foreground(fg)).Gap(u(6)).Align(geom.Center))
		}

		return ui.HStack(row...).Gap(u(18)).Align(geom.Center).
			PaddingInsets(geom.Insets{Left: u(pick(28, 16)), Right: u(pick(28, 16))}).
			MinHeight(u(pick(32, 28)))
	}

	h := u(pick(36, 30))
	row := []gift.View{glassPill(h, clock), fill()}
	for _, it := range items {
		row = append(row, glassPill(h, it.lead, ui.Text(it.label).FontSize(u(13)).Font(boldFont).Foreground(fg).MaxLines(1)))
	}

	return ui.HStack(row...).Gap(u(8)).Align(geom.Center).
		PaddingInsets(geom.Insets{Left: u(pick(24, 12)), Right: u(pick(24, 12))}).
		MinHeight(u(statusBarHeight()))
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

// homeScreen ist der Home-Bildschirm: oben groß die Uhrzeit und was
// ansteht, daneben der Start in den Kiosk; darunter Eingang, Drucker und
// Handy als Glaskarten; unten das Dock mit den App-Symbolen.
//
// Die Spalten teilen sich die Breite anteilig; auf kleinen Panels wird alles
// knapper, statt aus dem Bild zu laufen.
func (a *App) homeScreen(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)

	res := xgift.UseResource[homeData](ctx, "home")
	res.LoadKeyed([2]int{tick / 3, ctx.Read(st.photos)}, func() (homeData, error) { return a.loadHome() })
	d := res.Value()

	pad, gap := gutter(), spacing()
	inner := vw() - 2*pad - 2*gap
	inbox := inner * 0.46

	widgets := xgift.HStretch(
		xgift.Fill(a.inboxWidget(st, d, inbox)).Width(u(inbox)),
		xgift.Fill(a.printerWidget(st, d)).Flex(0.8),
		xgift.Fill(a.phoneWidget(d)).Flex(1.2),
	).Gap(u(gap)).Flex(1)

	return ui.VStack(
		ui.HStack(
			a.greeting(d),
			fill(),
			a.kioskWidget(st, d),
		).Align(geom.Top),
		widgets,
		ui.HStack(fill(), a.dock(st), fill()),
	).Gap(u(gap)).
		PaddingInsets(geom.Insets{Top: u(pick(8, 2)), Left: u(pad), Right: u(pad), Bottom: u(pick(20, 10))})
}

// greeting ist die Uhrzeit groß wie auf einem Sperrbildschirm und darunter,
// was ansteht.
func (a *App) greeting(d homeData) gift.View {
	line := "Keine neuen Fotos"
	switch {
	case d.unseen == 1:
		line = "1 neues Foto wartet im Eingang"
	case d.unseen > 1:
		line = fmt.Sprintf("%d neue Fotos warten im Eingang", d.unseen)
	}

	return ui.VStack(
		ui.Text(time.Now().Format("15:04")).FontSize(u(pick(84, 50))).Font(boldFont),
		ui.Text(line).FontSize(u(pick(19, 15))).Font(boldFont).Foreground(ui.ColorSecondaryLabel),
	).Gap(u(pick(2, 0))).Align(geom.Leading).PaddingInsets(geom.Insets{Left: u(4)})
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

func (a *App) inboxWidget(st *states, d homeData, width float32) gift.View {
	// So viele Vorschaubilder, wie mit mindestens 90 Punkten nebeneinander
	// passen, höchstens fünf; ihre Kante füllt dann die Zeile.
	inner := width - 2*pick(22, 14)
	gap := pick(12, 8)
	n := clamp(float32(int((inner+gap)/(90+gap))), 1, 5)
	edge := min((inner-gap*(n-1))/n, pick(170, 120), vh()*0.25)

	thumbs := []gift.View{}
	for i, loc := range d.inbox {
		if float32(i) >= n {
			break
		}

		thumbs = append(thumbs, thumb(loc.Path, u(edge)).CornerRadius(u(edge/8)).
			Shadow(ui.Shadow{Blur: u(14), OffsetY: u(5), Color: ui.RGBA(0, 0, 0, 45)}))
	}

	if len(thumbs) == 0 {
		thumbs = append(thumbs, muted("Noch nichts angekommen. Scanne den QR-Code mit dem Handy, um Fotos zu senden.", pick(16, 14)).MaxLines(3).Flex(1).MinHeight(u(edge)))
	}

	head := []gift.View{caps("EINGANG")}
	if d.unseen > 0 {
		head = append(head, ui.Badge(fmt.Sprintf("%d neu", d.unseen)).Color(blue))
	}

	head = append(head, fill(), link(pick2("Alle ansehen", "Alle"), func() { a.openLibrary(photo.ScopeInbox, "") }))

	footer := gift.View(ui.HStack(
		muted("Vom Handy und von der Kamera – gedruckt wird erst, wenn du wählst.", 14).MaxLines(2).Flex(1),
		primary("Auswählen und drucken", func() { a.openLibrary(photo.ScopeInbox, "") }),
	).Gap(u(16)).Align(geom.Center))
	if compact {
		footer = primary("Auswählen und drucken", func() { a.openLibrary(photo.ScopeInbox, "") })
	}

	return glassCard(
		ui.HStack(head...).Gap(u(10)).Align(geom.Center),
		ui.HStack(thumbs...).Gap(u(gap)),
		fill(),
		footer,
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

	detail := state + " · Citizen CZ-01"
	if p := d.printer.Problem(); p != "" && !d.settings.Printer.TestMode() {
		detail = p
	}

	return glassCard(
		caps("DRUCKER"),
		fill(),
		ui.VStack(
			ui.Text(fmt.Sprint(d.settings.PaperLeft)).FontSize(u(pick(64, 38))).Font(boldFont),
			muted(fmt.Sprintf("von %d Blatt", capacity), pick(15, 13)),
		).Align(geom.Center),
		ui.ProgressBar(frac).Tint(green).Frame(geom.Unbounded(), u(pick(8, 6))),
		ui.Text(detail).FontSize(u(pick(14, 12))).Font(boldFont).Foreground(color).MaxLines(2).Align(ui.AlignCenter),
		fill(),
		ui.HStack(fill(), link(pick2("Aufträge ansehen", "Aufträge"), func() { a.st.screen.Set(ScreenJobs) }), fill()),
	).Flex(1)
}

func (a *App) phoneWidget(d homeData) gift.View {
	edge := pick(150, clamp(vh()*0.2, 64, 120))

	hint := "Kamera öffnen, Code scannen, Fotos wählen. Sie landen im Eingang – gedruckt wird erst, wenn du hier auswählst."
	if compact {
		hint = "Code scannen, Fotos wählen – sie landen im Eingang."
	}

	// Ohne Upload-Dienst gibt es keinen Code; dann steht dort, warum, als
	// Text in voller Breite statt gequetscht in einem Quadrat. Der Code liegt
	// auf einer weißen Karte, damit jede Kamera ihn auf dem Glas findet.
	var code gift.View = ui.Box().Frame(0, 0)
	if d.address.URL != "" {
		code = ui.VStack(xgift.QRCode(d.address.URL, u(edge-pick(24, 14)))).
			Padding(u(pick(12, 7))).Background(white).CornerRadius(u(pick(20, 14)))
	} else {
		hint = orDash(d.address.Problem) + " Einrichten unter Einstellungen → Handy-Upload."
	}

	return glassCard(
		caps(pick2("VOM HANDY SENDEN", "VOM HANDY")),
		ui.HStack(
			code,
			muted(hint, pick(15, 13)).MaxLines(6).Flex(1),
		).Gap(u(gapIf(d.address.URL != "", pick(18, 10)))).Align(geom.Center).Flex(1),
	).Flex(1)
}

// dock ist die Leiste der App-Symbole unten in der Mitte, eine Glaskapsel
// wie bei iPadOS.
func (a *App) dock(st *states) gift.View {
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
		{"NAS", outline.Server, navy, func() { a.openLibrary(photo.ScopeAll, sourceNAS) }},
		{"Aufträge", outline.List, green, func() { st.screen.Set(ScreenJobs) }},
		{"USB-Stick", outline.ArchiveArrowDown, grey, func() { a.openLibrary(photo.ScopeAll, sourceUSB) }},
		{"Einstellungen", outline.Cog, ui.RGB(0x5E, 0x5E, 0x63), func() {
			st.settingsOpen.Set(false)
			st.screen.Set(ScreenSettings)
		}},
	}

	edge := pick(60, clamp(vh()*0.085, 40, 52))
	cells := make([]gift.View, 0, len(apps))
	for _, ap := range apps {
		face := ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}
		cells = append(cells, ui.Button(ui.VStack(
			xgift.IconTile(ap.sym, ap.face, u(edge)),
			ui.Text(ap.label).FontSize(u(pick(11.5, 10))).Font(boldFont).MaxLines(1),
		).Gap(u(pick(6, 3))).Align(geom.Center), ap.fn).
			Style(face).HoverStyle(face).
			PressedStyle(ui.ButtonStyle{Background: ui.Fade(ui.ColorLabel, 0.08), CornerRadius: u(16)}).
			Frame(u(edge+pick(52, 34)), geom.Unbounded()).
			Label(ap.label))
	}

	radius := u((edge + pick(46, 30)) / 2)
	return glassPane(radius, paneTint(),
		ui.HStack(cells...).Gap(u(pick(6, 2))).Align(geom.Center),
	).PaddingInsets(geom.Insets{Left: u(pick(14, 8)), Right: u(pick(14, 8)), Top: u(pick(10, 6)), Bottom: u(pick(8, 5))})
}

// kioskWidget ist der Start in den Kiosk: eine bernsteinfarben getönte
// Glaskapsel oben rechts.
func (a *App) kioskWidget(st *states, d homeData) gift.View {
	name := d.settings.EventTitle
	if name == "" {
		name = "Nächste Feier"
	}

	hint := "Aktiv bis zum Neustart"

	return glassPane(u(pick(28, 20)), ui.RGBA(40, 28, 6, 120),
		ui.Text("KIOSK-MODUS").FontSize(u(pick(12, 11))).Font(boldFont).Foreground(ui.RGB(0xFF, 0xCF, 0x73)),
		title(name, pick(21, 17)).Foreground(white).MaxLines(1),
		ui.HStack(
			ui.Text(hint).FontSize(u(pick(13, 12))).Foreground(ui.RGBA(255, 255, 255, 190)).MaxLines(1).Flex(1),
			filled("Kiosk starten", amber, ink, func() { a.openSheet(SheetKioskStart) }).MinHeight(u(pick(44, 38))),
		).Gap(u(12)).Align(geom.Center),
	).Gap(u(pick(6, 3))).
		Padding(u(pick(20, 12))).
		Frame(u(pick(360, 300)), geom.Unbounded())
}

// thumb ist ein quadratisches Vorschaubild eines Originals.
func thumb(path string, edge float32) ui.ImageView {
	return ui.Image(asset.File(path)).
		Fit(ui.FitCover).
		Frame(edge, edge).
		CornerRadius(edge / 12).
		Clip(true).
		Placeholder(ui.Fade(ui.ColorLabel, 0.08))
}

// gapIf ist gap, wenn es etwas zu trennen gibt, sonst 0.
func gapIf(cond bool, gap float32) float32 {
	if cond {
		return gap
	}

	return 0
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
