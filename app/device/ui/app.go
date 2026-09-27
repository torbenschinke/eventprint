package uidevice

import (
	"fmt"
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

	// nasSelected und usbSelected sind die Auswahl in fremden Quellen, deren
	// Bilder noch nicht auf der Box liegen.
	nasSelected []asset.ID
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

	// screen ist die Bemessung, nach der zuletzt aufgebaut wurde.
	screen display

	// dark ist das Erscheinungsbild, in dem zuletzt aufgebaut wurde.
	dark bool

	// nav merkt sich, woher der aktuelle Bildschirm kam, damit der Wechsel
	// als Bewegung in die richtige Richtung erscheint.
	nav struct{ current, previous Screen }
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

	// settingsOpen sagt auf kleinen Panels, ob in den Einstellungen das
	// Detail vorne liegt. Es steht hier und nicht in den Einstellungen
	// selbst, damit andere Bildschirme direkt in einen Abschnitt springen
	// können.
	settingsOpen *gift.State[bool]

	// fit zählt die Wechsel der Bemessung; jeder baut alles neu auf.
	fit *gift.State[int]
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

// SetScale gibt die Vergrößerung fest vor, statt sie aus dem Bildschirm zu
// bemessen. Für Bildschirme, die ihre Größe falsch melden.
func (a *App) SetScale(s float32) {
	if s > 0 {
		a.screen.fixed = s
		a.apply(a.screen)
	}
}

// SetPhysicalSize nennt die Größe des Panels in Millimetern, wie sie xrandr
// meldet. Ohne sie wird nach der Auflösung bemessen.
func (a *App) SetPhysicalSize(mm geom.Size) {
	d := a.screen
	d.physical = mm
	a.apply(d)
}

// Fit bemisst die Oberfläche für die Fläche des Fensters. Die Anwendung ruft
// es in jedem Takt; nur eine tatsächliche Änderung baut neu auf, und die
// passiert an der Box genau einmal, wenn das Vollbild steht.
//
// density ist die Dichte, mit der gift gerade zeichnet.
func (a *App) Fit(viewport geom.Size, density float32) {
	d := a.screen
	d.viewport, d.density = viewport, density
	a.apply(d)
}

func (a *App) apply(d display) {
	s, c := d.scale(), d.compact()
	g := design
	if d.viewport.W > 0 && d.viewport.H > 0 {
		g = geom.Sz(d.viewport.W/s, d.viewport.H/s)
	}

	changed := s != scale || c != compact || g != design
	a.screen = d
	scale, compact, design = s, c, g

	if !changed {
		return
	}

	slog.Info("display", "fit", d.String())
	if a.st == nil {
		return
	}

	// Galerien und ihre Füllung hängen an Kachelgrößen in Bildschirmpunkten.
	a.galleries = map[string]*ui.Gallery{}
	a.filled = map[string]any{}
	a.st.fit.Set(a.st.fit.Get() + 1)
}

// ApplyTheme setzt das Erscheinungsbild passend zur Betriebsart und zur
// Einstellung hell, dunkel oder automatisch.
func (a *App) ApplyTheme() {
	a.dark = a.wantDark()
	solid = a.wantSolid()
	setPalette(a.dark)

	if a.gapp == nil {
		return
	}

	k, _ := a.dev.Device.CurrentKiosk(a.dev.Subject())
	if !k.Active() {
		ui.SetTheme(a.gapp, homeTheme(a.dark))
		return
	}

	ui.SetTheme(a.gapp, kioskTheme(parseHex(k.Accent, amber)))
}

// wantDark meldet, ob gerade dunkel gezeichnet werden soll. Der Kiosk ist
// immer dunkel: Feiern sind abends, und ein heller Bildschirm blendet im
// gedämpften Licht.
func (a *App) wantDark() bool {
	k, _ := a.dev.Device.CurrentKiosk(sys())
	if k.Active() {
		return true
	}

	s, err := a.dev.Device.LoadSettings(sys())
	if err != nil {
		return false
	}

	return s.Appearance.Dark(time.Now())
}

// wantSolid meldet "Transparenz reduzieren".
func (a *App) wantSolid() bool {
	s, err := a.dev.Device.LoadSettings(sys())
	return err == nil && s.ReduceTransparency
}

// refreshTheme wechselt das Erscheinungsbild, wenn die Einstellung oder –
// bei "automatisch" – die Uhrzeit es verlangt. Der Takt ruft es regelmäßig.
func (a *App) refreshTheme() {
	if (a.wantDark() == a.dark && a.wantSolid() == solid) || a.st == nil {
		return
	}

	a.ApplyTheme()
	a.galleries = map[string]*ui.Gallery{}
	a.filled = map[string]any{}
	a.st.fit.Set(a.st.fit.Get() + 1)
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
		fit:           ctx.State("fit", 0),
		settingsOpen:  ctx.State("settingsOpen", false),
	}
	a.st = st
	a.startTicker(st)

	ctx.Read(st.mode)
	ctx.Read(st.sheet)
	fit := ctx.Read(st.fit)

	k, err := a.dev.Device.CurrentKiosk(a.dev.Subject())
	if err != nil {
		slog.Error("cannot read mode", "err", err)
	}

	var body gift.View
	if k.Active() {
		body = gift.Component(fmt.Sprint("kiosk@", fit), func(ctx *gift.Context) gift.View { return a.kioskScreen(ctx, st, k) })
	} else {
		body = gift.Component(fmt.Sprint("home@", fit), func(ctx *gift.Context) gift.View { return a.homeShell(ctx, st) })
	}

	return ui.Window(
		ui.Modal(
			ui.ZStack(xgift.Fill(body), gift.Component("toast", func(ctx *gift.Context) gift.View { return a.toast(ctx, st) })).Align(geom.Bottom),
			gift.Component("sheet", func(ctx *gift.Context) gift.View { return a.sheetView(ctx, st) }),
		).
			Presented(st.sheet.Get() != SheetNone).
			OnDismiss(func() { a.dismissSheet() }).
			// Abdunkeln statt aufhellen, auch im dunklen Erscheinungsbild.
			Scrim(ui.RGBA(0, 0, 0, 110)),
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
				xgift.Post(func() {
					st.tick.Set(st.tick.Get() + 1)
					a.refreshTheme()
				})
			}
		}()
	})
}

// modeChanged baut nach einem Wechsel der Betriebsart alles neu auf.
func (a *App) modeChanged() {
	a.selected, a.studio, a.nasSelected, a.usbSelected = nil, nil, nil, nil
	a.galleries = map[string]*ui.Gallery{}
	a.filled = map[string]any{}
	a.st.sheet.Set(SheetNone)
	a.st.screen.Set(ScreenHome)
	a.nav.current, a.nav.previous = ScreenHome, ScreenHome
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
