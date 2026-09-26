package uidevice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/usb"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// sheetCard ist die Fläche eines Dialogs.
func sheetCard(width float32, children ...gift.View) gift.View {
	return ui.VStack(children...).Gap(u(16)).Padding(u(28)).Frame(width, geom.Unbounded()).
		Background(ui.ColorSurface).CornerRadius(u(28))
}

// sheetView zeigt den aktuellen Dialog.
func (a *App) sheetView(ctx *gift.Context, st *states) gift.View {
	switch ctx.Read(st.sheet) {
	case SheetKioskStart:
		return gift.Component("kioskstart", func(ctx *gift.Context) gift.View { return a.kioskStartSheet(ctx, st) })
	case SheetPrinting:
		return gift.Component("printing", func(ctx *gift.Context) gift.View { return a.printingSheet(ctx, st) })
	case SheetExport:
		return gift.Component("export", func(ctx *gift.Context) gift.View { return a.exportSheet(ctx, st) })
	case SheetConfirmDelete:
		return a.deleteSheet(st)
	case SheetWifiPassword:
		return gift.Component("wifipw", func(ctx *gift.Context) gift.View { return a.wifiPasswordSheet(ctx, st) })
	case SheetPin:
		return gift.Component("pin", func(ctx *gift.Context) gift.View { return a.pinSheet(ctx, st) })
	case SheetOperator:
		return gift.Component("operator", func(ctx *gift.Context) gift.View { return a.operatorSheet(ctx, st) })
	case SheetKioskPrint:
		return gift.Component("kioskprint", func(ctx *gift.Context) gift.View { return a.kioskPrintSheet(ctx, st) })
	default:
		return ui.Box().Frame(1, 1)
	}
}

// kioskStartSheet prüft vor dem Start, ob das Gerät bereit ist.
func (a *App) kioskStartSheet(ctx *gift.Context, st *states) gift.View {
	res := xgift.UseResource[device.Report](ctx, "preflight")
	res.LoadOnce(func() (device.Report, error) {
		c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		return a.dev.Device.Preflight(a.dev.Subject(), c)
	})

	s, _ := a.dev.Device.LoadSettings(a.dev.Subject())
	name := s.EventTitle
	if name == "" {
		name = "Fotobox"
	}

	rows := []gift.View{}
	for _, c := range res.Value().Checks {
		sym, color := outline.CheckCircle, green
		switch c.Level {
		case device.LevelWarning:
			sym, color = outline.ExclamationCircle, orange
		case device.LevelError:
			sym, color = outline.CloseCircle, red
		}

		rows = append(rows, ui.HStack(
			ui.Icon(sym).Size(u(26)).Foreground(color),
			ui.VStack(title(c.Title, 16), muted(c.Detail, 13).MaxLines(2)).Gap(u(2)).Flex(1),
		).Gap(u(12)).Align(geom.Center).PaddingInsets(geom.Insets{Left: u(14), Right: u(14), Top: u(8), Bottom: u(8)}))
	}

	if !res.Loaded() {
		rows = append(rows, muted("Prüfe Drucker, Papier, Upload und Kamera …", 15).Padding(u(14)))
	}

	label := "Kiosk starten"
	if res.Value().Blocking() {
		label = "Trotzdem starten"
	}

	return sheetCard(u(600),
		ui.HStack(
			xgift.IconTile(outline.WandMagicSparkles, pink, u(52)),
			ui.VStack(title("Kiosk starten?", 24), body(name, 15)).Gap(u(2)),
		).Gap(u(14)).Align(geom.Center),
		ui.VStack(rows...).Background(ui.ColorBackground).CornerRadius(u(14)),
		ui.HStack(
			ui.Icon(outline.InfoCircle).Size(u(22)).Foreground(orange),
			body("Der Kiosk bleibt aktiv, bis die Box neu startet. Stecker ziehen und wieder einstecken bringt dich zurück in den Heimbetrieb. Mediathek und Einstellungen sind bis dahin verborgen.", 15).MaxLines(4).Flex(1),
		).Gap(u(12)).Padding(u(14)).Background(ui.RGB(0xFF, 0xF6, 0xE0)).CornerRadius(u(14)),
		ui.HStack(
			secondary("Abbrechen", func() { a.dismissSheet() }),
			primary(label, func() {
				if _, err := a.dev.Device.StartKiosk(a.dev.Subject(), device.StartKioskCmd{Title: name}); a.fail(err) {
					return
				}

				a.modeChanged()
			}).Flex(1),
		).Gap(u(12)),
	)
}

// deleteSheet bestätigt das endgültige Löschen.
func (a *App) deleteSheet(st *states) gift.View {
	n := len(a.selected)

	return sheetCard(u(480),
		title(fmt.Sprintf("%d %s löschen?", n, plural(n, "Foto", "Fotos")), 22),
		body("Die Originale werden von der Box entfernt. Das lässt sich nicht rückgängig machen.", 16).MaxLines(3),
		ui.HStack(
			secondary("Abbrechen", func() { a.dismissSheet() }),
			filled("Löschen", red, white, func() {
				if a.fail(a.dev.Photos.Delete(a.dev.Subject(), a.selected...)) {
					return
				}

				a.clearSelection()
				a.photosChanged()
				a.dismissSheet()
			}).Flex(1),
		).Gap(u(12)),
	)
}

// exportPlan ist das, was auf den Stick soll.
type exportPlan struct {
	folder string
	files  []usb.File
	drives []usb.Drive
}

// exportSheet kopiert Fotos auf einen USB-Stick.
func (a *App) exportSheet(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	progress := ctx.State("progress", "")
	report := ctx.State("report", "")
	busy := ctx.State("busy", false)

	res := xgift.UseResource[exportPlan](ctx, "plan")
	stamp := 0
	if len(res.Value().drives) == 0 {
		stamp = tick
	}

	res.LoadKeyed(stamp, func() (exportPlan, error) { return a.planExport() })
	plan := res.Value()

	var status gift.View
	switch {
	case ctx.Read(report) != "":
		status = body(report.Get(), 16).MaxLines(4)
	case ctx.Read(progress) != "":
		status = ui.VStack(body(progress.Get(), 16), ui.ProgressBar(0).Indeterminate().Frame(geom.Unbounded(), u(6))).Gap(u(8))
	case res.Err() != nil:
		status = body(res.Err().Error(), 16).Foreground(red).MaxLines(3)
	case len(plan.drives) == 0:
		status = ui.HStack(
			ui.Icon(outline.ArchiveArrowDown).Size(u(28)).Foreground(ui.ColorSecondaryLabel),
			body("Bitte einen USB-Stick einstecken. Er wird von selbst erkannt.", 16).MaxLines(2).Flex(1),
		).Gap(u(12)).Align(geom.Center)
	default:
		d := plan.drives[0]
		status = ui.HStack(
			ui.Icon(outline.ArchiveArrowDown).Size(u(28)).Foreground(green),
			ui.VStack(title(d.Title(), 17), muted(d.SizeText()+" · "+d.FSType, 13)).Gap(u(2)).Flex(1),
		).Gap(u(12)).Align(geom.Center)
	}

	actions := []gift.View{secondary("Schließen", func() { a.dismissSheet() })}
	if len(plan.drives) > 0 && report.Get() == "" {
		actions = append(actions, primary(fmt.Sprintf("%d %s kopieren", len(plan.files), plural(len(plan.files), "Foto", "Fotos")), func() {
			busy.Set(true)
			progress.Set("Kopiere …")
			drive := plan.drives[0].Path
			go func() {
				c, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()

				rep, err := a.dev.USB.Export(a.dev.Subject(), c, usb.ExportCmd{Drive: drive, Folder: plan.folder, Files: plan.files}, func(done, total int) {
					xgift.Post(func() { progress.Set(fmt.Sprintf("Kopiere %d von %d …", done, total)) })
				})
				xgift.Post(func() {
					busy.Set(false)
					progress.Set("")
					if a.fail(err) {
						return
					}

					report.Set(fmt.Sprintf("%d Fotos kopiert, %d waren schon da. Ordner: %s", rep.Copied, rep.Skipped, plan.folder))
				})
			}()
		}).Disabled(ctx.Read(busy) || len(plan.files) == 0).Flex(1))
	}

	if report.Get() != "" && len(plan.drives) > 0 {
		actions = append(actions, primary("Auswerfen", func() {
			a.eject(plan.drives[0].Path, func() { a.dismissSheet() })
		}).Flex(1))
	}

	return sheetCard(u(600),
		title("Auf USB-Stick kopieren", 24),
		muted(fmt.Sprintf("%d %s · Ordner „%s“", len(plan.files), plural(len(plan.files), "Foto", "Fotos"), plan.folder), 15).MaxLines(2),
		ui.VStack(status).Padding(u(16)).Background(ui.ColorBackground).CornerRadius(u(14)),
		ui.HStack(actions...).Gap(u(12)),
	)
}

// planExport stellt Dateien und Zielordner zusammen.
func (a *App) planExport() (exportPlan, error) {
	subject := a.dev.Subject()
	var plan exportPlan

	var ids []photo.ID
	name := "Auswahl"
	switch {
	case a.exportEvent != "":
		list, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: photo.ScopeEvent, Event: a.exportEvent})
		if err != nil {
			return plan, err
		}

		for _, p := range list {
			ids = append(ids, p.ID)
		}

		name = string(a.exportEvent)
		if s, err := a.dev.Device.LoadSettings(subject); err == nil {
			if e, ok := s.Event(a.exportEvent); ok {
				name = e.StartedAt.Local().Format("2006-01-02") + " " + e.Title
			}
		}
	case a.exportAll:
		list, err := a.dev.Photos.FindAll(subject, photo.Query{Scope: photo.ScopeAll})
		if err != nil {
			return plan, err
		}

		for _, p := range list {
			ids = append(ids, p.ID)
		}

		name = time.Now().Format("2006-01-02") + " Alle Fotos"
	default:
		ids = append(ids, a.selected...)
		name = time.Now().Format("2006-01-02 15-04") + " Auswahl"
	}

	locs, err := a.dev.Photos.Locate(subject, ids...)
	if err != nil {
		return plan, err
	}

	for _, l := range locs {
		plan.files = append(plan.files, usb.File{Name: l.ExportName, Path: l.Path})
	}

	plan.folder = "eventprint/" + strings.TrimSpace(name)

	c, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	plan.drives, err = a.dev.USB.Drives(subject, c)

	return plan, err
}
