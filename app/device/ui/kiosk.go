package uidevice

import (
	"fmt"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/camera"
	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/app/relay"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// kioskIdle schließt die Druckauswahl, wenn niemand mehr davorsteht. Sonst
// fände der nächste Gast das Foto eines anderen vor.
const kioskIdle = 30 * time.Second

// kioskData ist der Stand des Kiosk-Bildschirms.
type kioskData struct {
	photos   []photo.Location
	address  relay.Address
	queued   int
	printing bool
	camera   camera.Status
	paper    int
}

// kioskScreen ist der Bildschirm der Feier.
func (a *App) kioskScreen(ctx *gift.Context, st *states, k device.Kiosk) gift.View {
	tick := ctx.Read(st.tick)
	rev := ctx.Read(st.photos)

	res := xgift.UseResource[kioskData](ctx, "kiosk")
	res.LoadKeyed([2]int{tick, rev}, func() (kioskData, error) { return a.loadKiosk(k) })
	d := res.Value()

	g := a.gallery("kiosk")
	if a.refill("kiosk", locationKey(d.photos)) && res.Err() == nil {
		fillGallery(g, d.photos)
	}

	queue := gift.View(ui.Box().Frame(1, 1))
	if d.printing || d.queued > 0 {
		label := "Wird gedruckt"
		if d.queued > 1 {
			label = fmt.Sprintf("Wird gedruckt · noch %d in der Warteschlange", d.queued-1)
		}

		queue = ui.HStack(
			ui.ProgressBar(0).Indeterminate().Tint(ui.ColorAccent).Frame(u(80), u(6)),
			body(label, 15),
		).Gap(u(12)).Align(geom.Center).
			PaddingInsets(geom.Insets{Left: u(16), Right: u(16), Top: u(10), Bottom: u(10)}).
			Background(kioskCard).CornerRadius(u(24))
	}

	var grid gift.View
	if len(d.photos) == 0 {
		grid = ui.VStack(
			ui.Icon(outline.CameraPhoto).Size(u(64)).Foreground(kioskMuted),
			body("Noch keine Fotos. Das erste Bild von Kamera oder Handy erscheint hier.", 20).Foreground(kioskMuted).MaxLines(2),
		).Gap(u(16)).Align(geom.Center).Flex(1)
	} else {
		grid = ui.ImageGallery(g).
			Layout(brickRows(u(170)).Gap(u(14))).
			Tile(ui.TileStyle{CornerRadius: u(16), Palette: []ui.Color{kioskCard, kioskRaised}}).
			OnSelect(func(id asset.ID) {
				xgift.ShowSelection(g, nil)
				st.kioskPhoto.Set(photo.ID(id))
				st.kioskTemplate.Set(defaultTemplate(k))
				st.kioskCopies.Set(1)
				a.kioskOpened = time.Now()
				a.openSheet(SheetKioskPrint)
			}).
			Flex(1)
	}

	return ui.VStack(
		gift.Component("statusbar", func(ctx *gift.Context) gift.View { return a.statusBar(ctx, st, true, "Kiosk") }),
		ui.HStack(
			ui.VStack(
				title(k.Title, 46).Foreground(white).MaxLines(1),
				body("Tippe auf ein Foto, um es zu drucken", 20).Foreground(kioskMuted),
			).Gap(u(6)).Flex(1),
			queue,
		).Gap(u(16)).Align(geom.BottomLeading).PaddingInsets(geom.Insets{Top: u(18), Left: u(40), Right: u(40), Bottom: u(20)}),
		ui.VStack(xgift.HStretch(
			xgift.Fill(a.kioskInvite(d)).Width(u(360)),
			grow(grid),
		).Gap(u(32)).Flex(1)).PaddingInsets(geom.Insets{Left: u(40), Right: u(40), Bottom: u(32)}).Flex(1),
	).Background(kioskBg).Flex(1)
}

func defaultTemplate(k device.Kiosk) printing.TemplateID {
	if len(k.Layouts) > 0 {
		for _, t := range k.Layouts {
			if t == printing.TemplatePolaroid {
				return t
			}
		}

		return k.Layouts[0]
	}

	return printing.TemplatePolaroid
}

// loadKiosk liest, was der Kiosk zeigt – mit den Rechten des Gastes.
func (a *App) loadKiosk(k device.Kiosk) (kioskData, error) {
	var d kioskData
	subject := a.dev.Subject()

	list, err := a.dev.Photos.FindEvent(subject, k.Event, 500)
	if err != nil {
		return d, err
	}

	ids := make([]photo.ID, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.ID)
	}

	if d.photos, err = a.dev.Photos.Locate(subject, ids...); err != nil {
		return d, err
	}

	d.address, _ = a.dev.Relay.UploadAddress(subject, false)

	if seq, err := a.dev.Printing.FindAllJobs(subject); err == nil {
		cutoff := time.Now().Add(-time.Hour)
		for job, err := range seq {
			if err != nil || job.CreatedAt.Before(cutoff) {
				break
			}

			if !job.State.Done() {
				d.queued++
				d.printing = d.printing || job.State == printing.StatePrinting
			}
		}
	}

	if a.dev.Camera != nil {
		d.camera = a.dev.Camera.Status()
	}

	return d, nil
}

// kioskInvite ist die Einladung, eigene Fotos zu senden. Der QR-Code ist
// zugleich die verborgene Tür zur Betreuung: Gäste scannen ihn, sie tippen
// ihn nicht an. Fünfmal zügig getippt öffnet sich die PIN-Eingabe.
func (a *App) kioskInvite(d kioskData) gift.View {
	var code gift.View
	if d.address.URL != "" {
		code = xgift.QRCode(d.address.URL, u(236))
	} else {
		code = ui.VStack(
			title("Gerade nicht möglich", 17).Foreground(white),
			body(orDash(d.address.Problem), 14).Foreground(kioskMuted).MaxLines(3),
		).Gap(u(8)).Align(geom.Center).Frame(u(236), u(236)).Background(kioskRaised).CornerRadius(u(16)).Padding(u(16))
	}

	clear := ui.ButtonStyle{Background: ui.ColorClear, Border: noBorder}
	door := ui.Button(code, func() {
		if a.gate.Tap() {
			a.st.pin.Set("")
			a.openSheet(SheetPin)
		}
	}).Style(clear).HoverStyle(clear).PressedStyle(clear).Label("QR-Code zum Hochladen eigener Fotos")

	cam := "Kamera nicht verbunden"
	camColor := kioskMuted
	if d.camera.State == camera.StateConnected {
		cam, camColor = "Kamera bereit", ui.RGB(0x34, 0xC7, 0x59)
	}

	chip := func(dot ui.Color, label string) gift.View {
		return ui.HStack(
			ui.Box().Frame(u(8), u(8)).Background(dot).CornerRadius(u(4)),
			body(label, 14).Foreground(white),
		).Gap(u(6)).Align(geom.Center).
			PaddingInsets(geom.Insets{Left: u(12), Right: u(12), Top: u(6), Bottom: u(6)}).
			Background(kioskRaised).CornerRadius(u(16))
	}

	return ui.VStack(
		title("Eigene Fotos drucken", 24).Foreground(white),
		body("Scannen · Foto wählen · abholen", 16).Foreground(kioskMuted),
		door,
		fill(),
		ui.HStack(chip(camColor, cam)).Gap(u(8)),
	).Gap(u(14)).Align(geom.Center).Padding(u(26)).
		Background(kioskCard).CornerRadius(u(28))
}

// kioskPrintSheet fragt den Gast nach dem Layout und druckt.
func (a *App) kioskPrintSheet(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	id := ctx.Read(st.kioskPhoto)
	tpl := ctx.Read(st.kioskTemplate)
	copies := ctx.Read(st.kioskCopies)

	k, _ := a.dev.Device.CurrentKiosk(a.dev.Subject())

	left := kioskIdle - time.Since(a.kioskOpened)
	if left <= 0 {
		xgift.Post(func() { a.dismissSheet() })
	}

	_ = tick

	preview := xgift.UseResource[[]byte](ctx, "preview")
	preview.LoadKeyed([2]any{id, tpl}, func() ([]byte, error) {
		return a.dev.Printing.Preview(a.dev.Subject(), printing.PreviewCmd{Photos: []photo.ID{id}, Layout: tpl.Layout(), MaxEdge: int(u(700))})
	})

	var paper gift.View = ui.Box().Frame(u(300), u(450)).Background(kioskRaised)
	if data := preview.Value(); len(data) > 0 {
		paper = ui.Image(xgift.Memory(data)).Fit(ui.FitContain).Frame(u(300), u(450))
	}

	touch := func(fn func()) func() {
		return func() {
			a.kioskOpened = time.Now()
			fn()
		}
	}

	layouts := k.Layouts
	if len(layouts) == 0 {
		layouts = []printing.TemplateID{printing.TemplateFull, printing.TemplatePassepartout, printing.TemplatePolaroid}
	}

	cards := []gift.View{}
	for _, t := range layouts {
		border := ui.Border{Width: u(3), Color: kioskRaised}
		if t == tpl {
			border = ui.Border{Width: u(3), Color: ui.ColorAccent}
		}

		style := ui.ButtonStyle{Background: kioskRaised, Border: border, CornerRadius: u(22)}
		cards = append(cards, ui.Button(
			ui.Text(printing.TemplateByID(t).Name).FontSize(u(20)).Font(boldFont).Foreground(white),
			touch(func() { st.kioskTemplate.Set(t) }),
		).Style(style).HoverStyle(style).PressedStyle(style).MinHeight(u(96)).Flex(1))
	}

	maxCopies := max(k.MaxCopies, 1)

	return ui.HStack(
		ui.VStack(paper).Align(geom.Center).Padding(u(30)).Background(kioskRaised).CornerRadius(u(22)),
		ui.VStack(
			ui.HStack(
				title("Wie soll dein Foto aussehen?", 32).Foreground(white).Flex(1),
				body(fmt.Sprintf("schließt in %d s", int(left.Seconds())), 14).Foreground(kioskMuted),
			).Align(geom.Center),
			ui.HStack(cards...).Gap(u(16)),
			ui.HStack(
				title("Anzahl", 20).Foreground(white).Flex(1),
				xgift.Stepper(copies, 1, maxCopies, u(24), func(v int) { touch(func() { st.kioskCopies.Set(v) })() }),
				body(fmt.Sprintf("höchstens %d", maxCopies), 14).Foreground(kioskMuted),
			).Gap(u(16)).Align(geom.Center).Padding(u(16)).Background(kioskRaised).CornerRadius(u(18)),
			fill(),
			ui.HStack(
				filled("Abbrechen", kioskRaised, white, func() { a.dismissSheet() }).MinHeight(u(72)),
				filled("Drucken", ui.ColorAccent, ink, func() {
					_, err := a.dev.Printing.PrintSimple(a.dev.Subject(), printing.SimpleCmd{Photo: id, Template: tpl, Copies: copies})
					if a.fail(err) {
						return
					}

					a.dismissSheet()
					a.show("Dein Foto wird gedruckt. Abholen am Drucker.")
				}).MinHeight(u(72)).Flex(1),
			).Gap(u(16)),
		).Gap(u(22)).Flex(1),
	).Gap(u(40)).Padding(u(36)).Frame(u(1140), u(620)).Background(kioskCard).CornerRadius(u(32))
}

// pinSheet fragt die Betreuer-PIN ab.
func (a *App) pinSheet(ctx *gift.Context, st *states) gift.View {
	pin := ctx.Read(st.pin)

	// So breit wie das Tastenfeld und nicht breiter: drei Tasten, zwei
	// Abstände, der Innenrand des Dialogs.
	return sheetCard(3*u(72)+2*u(24)+2*u(28),
		ui.HStack(fill(), ui.VStack(title("Betreuung", 24), muted("PIN eingeben", 15)).Gap(u(4)).Align(geom.Center), fill()),
		xgift.PinPad(device.PinLength, pin, u(72), st.pin.Set, func(full string) {
			err := a.dev.Device.Unlock(a.dev.Subject(), full)
			st.pin.Set("")
			if a.fail(err) {
				return
			}

			a.openSheet(SheetOperator)
		}),
		ui.HStack(fill(), link("Abbrechen", func() { a.dismissSheet() }), fill()),
	)
}

// operatorSheet ist das Menü der Betreuung im Kiosk.
func (a *App) operatorSheet(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	k, _ := a.dev.Device.CurrentKiosk(a.dev.Subject())

	res := xgift.UseResource[jobsData](ctx, "jobs")
	res.LoadKeyed(tick/2, func() (jobsData, error) { return a.loadJobs("", 8) })
	d := res.Value()

	rows := []gift.View{}
	for _, job := range d.jobs {
		rows = append(rows, a.jobRow(job, d.thumbs, func() {}))
	}

	return sheetCard(u(760),
		ui.HStack(title("Betreuung", 26).Flex(1), secondary("Sperren", func() {
			a.dev.Relock()
			a.dismissSheet()
		})).Align(geom.Center),
		a.printerCard(d, func() {}),
		ui.VScroll(ui.VStack(rows...)).MaxHeight(u(240)).Background(ui.ColorBackground).CornerRadius(u(14)),
		ui.HStack(
			secondary("Fotos auf USB-Stick kopieren", func() {
				a.exportEvent = k.Event
				a.exportAll = false
				a.openSheet(SheetExport)
			}),
			fill(),
			filled("Kiosk beenden", red, white, func() {
				if a.fail(a.dev.Device.StopKiosk(a.dev.Subject())) {
					return
				}

				a.modeChanged()
			}),
		).Gap(u(12)).Align(geom.Center),
		muted("Die Freischaltung endet nach zehn Minuten von selbst. Ausschalten der Box führt ebenfalls zurück in den Heimbetrieb.", 13).MaxLines(2),
	)
}
