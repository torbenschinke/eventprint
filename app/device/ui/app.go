package uidevice

import (
	"log/slog"
	"sync"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"

	cfgdevice "github.com/torbenschinke/eventprint/app/device/cfg"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// Screen ist ein Bildschirm des Heimbetriebs.
type Screen int

const (
	ScreenHome Screen = iota
	ScreenLibrary
	ScreenStudio
	ScreenJobs
	ScreenSettings
)

// Sheet ist ein Dialog über dem aktuellen Bildschirm.
type Sheet int

const (
	SheetNone Sheet = iota
	SheetKioskStart
	SheetPrinting
	SheetPin
	SheetOperator
	SheetKioskPrint
	SheetExport
	SheetConfirmDelete
	SheetWifiPassword
)

// App ist die Oberfläche. Ihre Felder gehören dem UI-Thread.
type App struct {
	dev  *cfgdevice.Device
	gapp *gift.App

	// selected ist die Auswahl in der Mediathek, studio die Fotos im
	// Druck-Studio. Beides sind Listen und damit keine gift-Zustände; eine
	// Änderung meldet st.selection.
	selected []photo.ID
	studio   []photo.ID

	// usbSelected ist die Auswahl in einer fremden Quelle, deren Bilder noch
	// nicht auf der Box liegen.
	usbSelected []asset.ID

	// exportEvent und exportAll wählen aus, was auf den USB-Stick geht; ohne
	// beides die Auswahl der Mediathek.
	exportEvent photo.EventID
	exportAll   bool

	// kioskOpened ist der Zeitpunkt der letzten Berührung in der
	// Druckauswahl des Kiosks.
	kioskOpened time.Time

	galleries map[string]*ui.Gallery
	filled    map[string]any
	gate      *xgift.TapGate

	tickOnce sync.Once
	st       *states
}

// states sind die Zustände, die mehrere Bildschirme lesen. Sie entstehen in
// der Wurzel und werden als Zeiger weitergereicht, damit jeder Bildschirm nur
// die abonniert, die er wirklich liest.
type states struct {
	tick      *gift.State[int]
	mode      *gift.State[int]
	screen    *gift.State[Screen]
	sheet     *gift.State[Sheet]
	message   *gift.State[string]
	selection *gift.State[int]
	photos    *gift.State[int]

	libScope  *gift.State[photo.Scope]
	libEvent  *gift.State[photo.EventID]
	libSource *gift.State[string]

	layout    *gift.State[printing.Layout]
	copies    *gift.State[int]
	active    *gift.State[int]
	studioTab *gift.State[int]

	batch    *gift.State[printing.BatchID]
	settings *gift.State[int]
	pin      *gift.State[string]

	kioskPhoto    *gift.State[photo.ID]
	kioskTemplate *gift.State[printing.TemplateID]
	kioskCopies   *gift.State[int]

	wifiSSID *gift.State[string]
}

// New erzeugt die Oberfläche über dem verdrahteten Gerät.
func New(dev *cfgdevice.Device, gapp *gift.App) *App {
	return &App{
		dev:       dev,
		gapp:      gapp,
		galleries: map[string]*ui.Gallery{},
		gate:      xgift.NewTapGate(5, 2*time.Second),
	}
}

// SetApp verbindet die Oberfläche mit der gift-Anwendung, wenn diese erst
// nach ihr entsteht – etwa in einem Test, dessen Harness die Anwendung
// selbst anlegt.
func (a *App) SetApp(gapp *gift.App) { a.gapp = gapp }

// ApplyTheme setzt das Erscheinungsbild passend zur Betriebsart.
func (a *App) ApplyTheme() {
	if a.gapp == nil {
		return
	}

	k, _ := a.dev.Device.CurrentKiosk(a.dev.Subject())
	if !k.Active() {
		ui.SetTheme(a.gapp, homeTheme())
		return
	}

	ui.SetTheme(a.gapp, kioskTheme(parseHex(k.Accent, amber)))
}

// Root ist die Wurzel des Bildschirms.
func (a *App) Root(ctx *gift.Context) gift.View {
	st := &states{
		tick:          ctx.State("tick", 0),
		mode:          ctx.State("mode", 0),
		screen:        ctx.State("screen", ScreenHome),
		sheet:         ctx.State("sheet", SheetNone),
		message:       ctx.State("message", ""),
		selection:     ctx.State("selection", 0),
		photos:        ctx.State("photos", 0),
		libScope:      ctx.State("libScope", photo.ScopeInbox),
		libEvent:      ctx.State("libEvent", photo.EventID("")),
		libSource:     ctx.State("libSource", ""),
		layout:        ctx.State("layout", printing.DefaultLayout()),
		copies:        ctx.State("copies", 1),
		active:        ctx.State("active", 0),
		studioTab:     ctx.State("studioTab", 0),
		batch:         ctx.State("batch", printing.BatchID("")),
		settings:      ctx.State("settings", 0),
		pin:           ctx.State("pin", ""),
		kioskPhoto:    ctx.State("kioskPhoto", photo.ID("")),
		kioskTemplate: ctx.State("kioskTemplate", printing.TemplatePolaroid),
		kioskCopies:   ctx.State("kioskCopies", 1),
		wifiSSID:      ctx.State("wifiSSID", ""),
	}
	a.st = st
	a.startTicker(st)

	ctx.Read(st.mode)
	ctx.Read(st.sheet)

	k, err := a.dev.Device.CurrentKiosk(a.dev.Subject())
	if err != nil {
		slog.Error("cannot read mode", "err", err)
	}

	var body gift.View
	if k.Active() {
		body = gift.Component("kiosk", func(ctx *gift.Context) gift.View { return a.kioskScreen(ctx, st, k) })
	} else {
		body = gift.Component("home", func(ctx *gift.Context) gift.View { return a.homeShell(ctx, st) })
	}

	return ui.Window(
		ui.Modal(
			ui.ZStack(xgift.Fill(body), gift.Component("toast", func(ctx *gift.Context) gift.View { return a.toast(ctx, st) })).Align(geom.Bottom),
			gift.Component("sheet", func(ctx *gift.Context) gift.View { return a.sheetView(ctx, st) }),
		).
			Presented(st.sheet.Get() != SheetNone).
			OnDismiss(func() { a.dismissSheet() }),
		ui.OnScreenKeyboard(),
	).Align(geom.Bottom)
}

// startTicker lässt die Uhr gehen. Bildschirme mit laufenden Vorgängen –
// Druckfortschritt, neue Fotos im Eingang, Uhrzeit – lesen den Takt und
// frischen sich damit auf, ohne dass jemand tippt.
func (a *App) startTicker(st *states) {
	a.tickOnce.Do(func() {
		go func() {
			t := time.NewTicker(2 * time.Second)
			defer t.Stop()
			for range t.C {
				xgift.Post(func() { st.tick.Set(st.tick.Get() + 1) })
			}
		}()
	})
}

// modeChanged baut nach einem Wechsel der Betriebsart alles neu auf.
func (a *App) modeChanged() {
	a.selected, a.studio, a.usbSelected = nil, nil, nil
	a.galleries = map[string]*ui.Gallery{}
	a.filled = map[string]any{}
	a.st.sheet.Set(SheetNone)
	a.st.screen.Set(ScreenHome)
	a.st.pin.Set("")
	a.ApplyTheme()
	a.st.mode.Set(a.st.mode.Get() + 1)
}

// show zeigt einen Hinweis am unteren Rand.
func (a *App) show(msg string) {
	a.st.message.Set(msg)
}

// fail zeigt einen Fehler, falls es einen gibt, und meldet, ob es einen gab.
func (a *App) fail(err error) bool {
	if err == nil {
		return false
	}

	slog.Error("ui action failed", "err", err)
	a.show(err.Error())

	return true
}

func (a *App) openSheet(s Sheet) { a.st.sheet.Set(s) }

func (a *App) dismissSheet() {
	// Der Druckfortschritt ist ein Hinweis, kein Vorgang: Wegtippen lässt
	// den Druck weiterlaufen. Die PIN wird dagegen verworfen.
	a.st.pin.Set("")
	a.st.sheet.Set(SheetNone)
}

// toast zeigt den letzten Hinweis, bis jemand ihn wegtippt.
func (a *App) toast(ctx *gift.Context, st *states) gift.View {
	msg := ctx.Read(st.message)
	if msg == "" {
		return ui.Box().Frame(1, 1)
	}

	return floating(ui.Button(
		ui.HStack(
			ui.Text(msg).FontSize(u(15)).Foreground(white).MaxLines(3).Flex(1),
			ui.Text("OK").FontSize(u(15)).Font(boldFont).Foreground(amber),
		).Gap(u(16)).Align(geom.Center),
		func() { st.message.Set("") },
	).
		Style(ui.ButtonStyle{Background: ui.RGBA(0x1C, 0x1C, 0x1E, 0xF0), CornerRadius: u(16)}).
		HoverStyle(ui.ButtonStyle{Background: ui.RGBA(0x1C, 0x1C, 0x1E, 0xF0), CornerRadius: u(16)}).
		PaddingInsets(geom.Insets{Top: u(14), Bottom: u(14), Left: u(20), Right: u(20)}).
		MaxWidth(u(720)))
}

// parseHex liest eine Farbe #RRGGBB, im Zweifel die Vorgabe.
func parseHex(s string, fallback ui.Color) ui.Color {
	if len(s) != 7 || s[0] != '#' {
		return fallback
	}

	v := func(i int) (uint8, bool) {
		var n uint8
		for _, c := range s[i : i+2] {
			n <<= 4
			switch {
			case c >= '0' && c <= '9':
				n |= uint8(c - '0')
			case c >= 'a' && c <= 'f':
				n |= uint8(c-'a') + 10
			case c >= 'A' && c <= 'F':
				n |= uint8(c-'A') + 10
			default:
				return 0, false
			}
		}

		return n, true
	}

	r, ok1 := v(1)
	g, ok2 := v(3)
	b, ok3 := v(5)
	if !ok1 || !ok2 || !ok3 {
		return fallback
	}

	return ui.RGB(r, g, b)
}
