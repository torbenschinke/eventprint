package uidevice

import (
	"fmt"
	"slices"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/icon/outline"
	"github.com/worldiety/gift/ui"

	"github.com/torbenschinke/eventprint/app/device"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
	"github.com/torbenschinke/eventprint/pkg/xgift"
)

// jobsData ist der Stand der Aufträge.
type jobsData struct {
	jobs     []printing.Job
	thumbs   map[photo.ID]string
	printer  printing.PrinterStatus
	settings device.Settings
}

// loadJobs liest die jüngsten Aufträge, auf Wunsch nur die eines Batches.
func (a *App) loadJobs(batch printing.BatchID, limit int) (jobsData, error) {
	d := jobsData{thumbs: map[photo.ID]string{}}
	subject := a.dev.Subject()

	seq, err := a.dev.Printing.FindAllJobs(subject)
	if err != nil {
		return d, err
	}

	var ids []photo.ID
	for job, err := range seq {
		if err != nil {
			return d, err
		}

		if batch != "" && job.Batch != batch {
			continue
		}

		d.jobs = append(d.jobs, job)
		if len(job.Photos) > 0 {
			ids = append(ids, job.Photos[0])
		}

		if limit > 0 && len(d.jobs) >= limit {
			break
		}
	}

	// Ein Batch wird von vorn nach hinten gedruckt; so liest man ihn auch.
	if batch != "" {
		slices.SortFunc(d.jobs, func(x, y printing.Job) int { return x.Sheet - y.Sheet })
	}

	if locs, err := a.dev.Photos.Locate(subject, ids...); err == nil {
		for _, l := range locs {
			d.thumbs[l.Photo.ID] = l.Path
		}
	}

	d.printer, _ = a.dev.Printing.Diagnose(subject)
	if s, err := a.dev.Device.LoadSettings(subject); err == nil {
		d.settings = s
	}

	return d, nil
}

// jobsScreen zeigt Drucker und Aufträge.
func (a *App) jobsScreen(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	rev := ctx.State("rev", 0)

	res := xgift.UseResource[jobsData](ctx, "jobs")
	res.LoadKeyed([2]int{tick, ctx.Read(rev)}, func() (jobsData, error) { return a.loadJobs("", 60) })

	d := res.Value()
	refresh := func() { rev.Set(rev.Get() + 1) }

	rows := []gift.View{}
	for _, job := range d.jobs {
		rows = append(rows, a.jobRow(job, d.thumbs, refresh))
	}

	if len(rows) == 0 {
		rows = append(rows, ui.Row("Noch keine Aufträge").Subtitle("Gedruckte Blätter erscheinen hier."))
	}

	return ui.VScroll(ui.VStack(
		ui.HStack(
			iconButton(outline.Home, "Home", ui.ColorAccent, func() { st.screen.Set(ScreenHome) }),
			title("Aufträge", 30).Flex(1),
		).Gap(u(12)).Align(geom.Center),
		a.printerCard(d, refresh),
		section("DRUCKAUFTRÄGE", rows...),
	).Gap(u(20)).Padding(u(32)).MaxWidth(u(900))).Flex(1)
}

// printerCard zeigt den Zustand des Druckers und die Handgriffe dazu.
func (a *App) printerCard(d jobsData, refresh func()) gift.View {
	state, color := "Bereit", greenText
	detail := d.printer.Queue
	switch {
	case d.settings.Printer.TestMode():
		state, color, detail = "Testbetrieb", orange, "Kein Drucker eingerichtet – Aufträge laufen durch, gedruckt wird nichts."
	case !d.printer.OK():
		state, color, detail = "Gestört", red, d.printer.Problem()
	}

	actions := []gift.View{
		secondary("Neues Papier eingelegt", func() {
			if _, err := a.dev.Device.RefillPaper(a.dev.Subject()); !a.fail(err) {
				a.show("Papiervorrat zurückgesetzt.")
				refresh()
			}
		}),
	}

	if !d.printer.Enabled && d.printer.Exists {
		actions = append(actions, primary("Drucker freigeben", func() {
			if !a.fail(a.dev.Printing.Resume(a.dev.Subject())) {
				a.show("Der Drucker läuft wieder.")
				refresh()
			}
		}))
	}

	return card(
		ui.HStack(
			xgift.IconTile(outline.Printer, grey, u(40)),
			ui.VStack(title("Citizen CZ-01", 19), muted(orDash(detail), 14).MaxLines(3)).Gap(u(2)).Flex(1),
			ui.Text(state).FontSize(u(15)).Font(boldFont).Foreground(color),
		).Gap(u(14)).Align(geom.Center),
		ui.HStack(
			body(fmt.Sprintf("%d von %d Blatt übrig", d.settings.PaperLeft, max(d.settings.PaperCapacity, 1)), 15).Flex(1),
			ui.HStack(actions...).Gap(u(10)),
		).Gap(u(12)).Align(geom.Center),
	)
}

func describeJob(job printing.Job) string {
	name := string(job.Layout.Format)
	for _, f := range printing.Formats() {
		if f.ID == job.Layout.Normalized().Format {
			name = f.Name
		}
	}

	for _, d := range printing.Designs() {
		if d.ID == job.Layout.Normalized().Design {
			name += " · " + d.Name
		}
	}

	if job.Sheets > 1 {
		name = fmt.Sprintf("Blatt %d von %d · %s", job.Sheet, job.Sheets, name)
	}

	return name
}

func stateColor(s printing.State) ui.Color {
	switch s {
	case printing.StateDone:
		return greenText
	case printing.StateFailed:
		return red
	case printing.StatePrinting:
		return blueText
	default:
		return ui.ColorSecondaryLabel
	}
}

// jobRow ist eine Zeile der Auftragsliste.
func (a *App) jobRow(job printing.Job, thumbs map[photo.ID]string, refresh func()) gift.View {
	var img gift.View = ui.Box().Frame(u(44), u(44)).Background(ui.ColorBackground).CornerRadius(u(6))
	if len(job.Photos) > 0 {
		if p, ok := thumbs[job.Photos[0]]; ok {
			img = thumb(p, u(44))
		}
	}

	var action gift.View = ui.Box().Frame(1, 1)
	switch {
	case job.State == printing.StateFailed:
		action = link("Wiederholen", func() {
			if !a.fail(a.dev.Printing.Retry(a.dev.Subject(), job.ID)) {
				refresh()
			}
		})
	case !job.State.Done():
		action = link("Abbrechen", func() {
			if !a.fail(a.dev.Printing.Cancel(a.dev.Subject(), job.ID)) {
				refresh()
			}
		})
	}

	sub := job.CreatedAt.Local().Format("02.01. 15:04")
	if job.State == printing.StateFailed && job.Message != "" {
		sub += " · " + job.Message
	}

	return ui.HStack(
		img,
		ui.VStack(body(describeJob(job), 16).MaxLines(1), muted(sub, 13).MaxLines(2)).Gap(u(2)).Flex(1),
		ui.Text(job.State.String()).FontSize(u(15)).Font(boldFont).Foreground(stateColor(job.State)),
		action,
	).Gap(u(12)).Align(geom.Center).PaddingInsets(geom.Insets{Left: u(14), Right: u(14), Top: u(6), Bottom: u(6)}).MinHeight(u(58))
}

// printingSheet zeigt den Fortschritt eines Druckvorgangs.
//
// Er ist ein Hinweis und kein Warten: Wegtippen lässt den Druck weiterlaufen.
// Bleibt ein Blatt am Papier hängen, sagt er, was zu tun ist, und bietet das
// Weiterdrucken direkt an – ohne Umweg über die Auftragsliste.
func (a *App) printingSheet(ctx *gift.Context, st *states) gift.View {
	tick := ctx.Read(st.tick)
	batch := ctx.Read(st.batch)
	rev := ctx.State("rev", 0)

	res := xgift.UseResource[jobsData](ctx, "batch")
	res.LoadKeyed([3]any{batch, tick, ctx.Read(rev)}, func() (jobsData, error) { return a.loadJobs(batch, 0) })
	d := res.Value()

	done, failed, open := 0, 0, 0
	var current printing.Job
	for _, j := range d.jobs {
		switch {
		case j.State == printing.StateDone:
			done++
		case j.State == printing.StateFailed:
			failed++
		default:
			open++
			if current.ID == "" || j.State == printing.StatePrinting {
				current = j
			}
		}
	}

	total := max(len(d.jobs), 1)
	head, sub := "Wird gedruckt", fmt.Sprintf("Blatt %d von %d", min(done+1, total), total)
	switch {
	case open == 0 && failed == 0 && res.Loaded():
		head, sub = "Fertig gedruckt", fmt.Sprintf("%d %s", done, plural(done, "Blatt", "Blätter"))
	case failed > 0 && open == 0:
		head, sub = "Nicht alles gedruckt", fmt.Sprintf("%d von %d Blatt fehlen", failed, total)
	}

	var preview gift.View = ui.Box().Frame(u(128), u(192)).Background(ui.ColorBackground)
	if current.ID == "" && len(d.jobs) > 0 {
		current = d.jobs[len(d.jobs)-1]
	}

	if current.ID != "" && len(current.Photos) > 0 {
		if p, ok := d.thumbs[current.Photos[0]]; ok {
			preview = thumb(p, u(128))
		}
	}

	problem := ""
	if !d.printer.OK() && !d.settings.Printer.TestMode() {
		problem = d.printer.Problem()
	}

	rows := []gift.View{}
	for _, j := range d.jobs {
		rows = append(rows, a.jobRow(j, d.thumbs, func() { rev.Set(rev.Get() + 1) }))
	}

	actions := []gift.View{}
	if open > 0 {
		actions = append(actions, filled("Abbrechen", ui.RGB(0xFD, 0xEC, 0xEC), red, func() {
			var ids []printing.JobID
			for _, j := range d.jobs {
				if !j.State.Done() {
					ids = append(ids, j.ID)
				}
			}

			if !a.fail(a.dev.Printing.Cancel(a.dev.Subject(), ids...)) {
				rev.Set(rev.Get() + 1)
			}
		}))
	}

	if problem != "" || failed > 0 {
		actions = append(actions, secondary("Eingelegt – weiter drucken", func() {
			subject := a.dev.Subject()
			_ = a.dev.Printing.Resume(subject)
			for _, j := range d.jobs {
				if j.State == printing.StateFailed && j.Reason != "canceled-by-user" {
					if a.fail(a.dev.Printing.Retry(subject, j.ID)) {
						return
					}
				}
			}

			rev.Set(rev.Get() + 1)
		}))
	}

	actions = append(actions, primary("Fertig", func() { a.dismissSheet() }).Flex(1))

	children := []gift.View{
		ui.HStack(
			preview,
			ui.VStack(
				title(head, 26),
				body(sub, 16),
				ui.ProgressBar(float64(done)/float64(total)).Tint(green).Frame(geom.Unbounded(), u(8)),
				muted("Der CZ-01 zieht jedes Blatt viermal durch: Gelb, Magenta, Cyan und die Schutzschicht. Bitte nicht daran ziehen.", 14).MaxLines(3),
			).Gap(u(10)).Flex(1),
		).Gap(u(24)),
	}

	if problem != "" {
		children = append(children, ui.HStack(
			ui.Icon(outline.ExclamationCircle).Size(u(24)).Foreground(orange),
			body(problem+" – neues Papier und Farbband einlegen, dann weiter drucken. Kein Blatt wird doppelt gedruckt.", 15).MaxLines(4).Flex(1),
		).Gap(u(12)).Padding(u(14)).Background(ui.RGB(0xFF, 0xF1, 0xE0)).CornerRadius(u(14)))
	}

	children = append(children,
		ui.VScroll(ui.VStack(rows...)).MaxHeight(u(220)).Background(ui.ColorBackground).CornerRadius(u(14)),
		ui.HStack(actions...).Gap(u(12)),
	)

	return ui.VStack(children...).Gap(u(18)).Padding(u(28)).Frame(u(640), geom.Unbounded()).
		Background(ui.ColorSurface).CornerRadius(u(28))
}
