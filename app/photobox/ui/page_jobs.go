package uiphotobox

import (
	"strconv"
	"time"

	"go.wdy.de/nago/presentation/core"
	"go.wdy.de/nago/presentation/ui"
	"go.wdy.de/nago/presentation/ui/alert"

	"github.com/torbenschinke/eventprint/app/printing"
)

// jobsRefreshRate hält die Statusseite aktuell, während der Drucker arbeitet.
const jobsRefreshRate = 2 * time.Second

// PageJobs zeigt den Zustand der Druckwarteschlange.
//
// Die Seite richtet sich an denjenigen, der die Fotobox betreut: Sie
// beantwortet die zwei Fragen, die an einem Abend tatsächlich auftreten –
// "kommt mein Bild noch?" und "warum kommt es nicht?".
func PageJobs(wnd core.Window, opts Options) core.View {
	var (
		rows    []core.View
		pending int
	)

	all, err := opts.Printing.FindAllJobs(wnd.Subject())
	if err != nil {
		return alert.BannerError(err)
	}

	for job, err := range all {
		if err != nil {
			return alert.BannerError(err)
		}

		if !job.State.Done() {
			pending++
		}

		rows = append(rows, jobRow(wnd, opts, job))
	}

	content := ui.VStack(
		alert.BannerMessages(wnd),

		testModeHint(wnd, opts),
		printerProblemHint(wnd, opts),

		ui.HStack(
			ui.VStack(
				ui.Text("Druckstatus").Font(ui.TitleLarge),
				ui.Text("Drucker: "+opts.Printing.Printer.Name()).Font(ui.BodySmall),
			).Alignment(ui.Leading),
			ui.Spacer(),
			pendingBadge(pending),
		).FullWidth(),

		ui.If(len(rows) == 0, emptyHint(
			"Keine Druckaufträge",
			"Sobald ein Foto gedruckt wird, taucht der Auftrag hier auf.",
		)),

		ui.VStack(rows...).Gap(ui.L8).FullWidth(),
	).
		Gap(ui.L24).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Frame(ui.Frame{}.FullWidth())

	return ui.RedrawAtFixedRate(wnd, jobsRefreshRate, content)
}

// testModeHint weist deutlich darauf hin, dass noch kein Drucker eingerichtet
// ist.
//
// Ohne diesen Hinweis steht lediglich beiläufig "Testmodus" in der Kopfzeile –
// wer die Fotobox aufbaut, sucht dann den Fehler beim Drucker statt bei der
// Konfiguration.
func testModeHint(wnd core.Window, opts Options) core.View {
	if !printing.IsTestMode(opts.Printing.Printer) {
		return nil
	}

	// Die Einstellungsseite liegt im Admin-Center. Gästen wird sie nicht
	// angeboten, sie könnten sie ohnehin nicht öffnen.
	var action core.View
	if wnd.Subject().Valid() && opts.PrinterSettings != "" {
		action = ui.PrimaryButton(func() {
			wnd.Navigation().ForwardTo(opts.PrinterSettings, opts.PrinterSettingsParams)
		}).Title("Drucker einrichten")
	} else {
		action = ui.Text("Zum Einrichten als Betreuer anmelden.").Font(ui.BodySmall)
	}

	return ui.VStack(
		ui.Text("Es ist kein Drucker eingerichtet").Font(ui.TitleMedium),
		ui.Text("Druckaufträge werden vollständig verarbeitet, aber nicht ausgegeben. Wähle in den Einstellungen die CUPS-Warteschlange des Fotodruckers.").
			Font(ui.BodyMedium),
		action,
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.FullWidth())
}

// printerProblemHint meldet, wenn CUPS den Drucker nicht bereit sieht.
//
// Das deckt die Fälle ab, die sonst nur im Terminal sichtbar wären:
// gelöschte Warteschlange, angehaltener Drucker, gestoppte Annahme sowie die
// Zustandsmeldung des Geräts – etwa "Out of paper".
func printerProblemHint(wnd core.Window, opts Options) core.View {
	if printing.IsTestMode(opts.Printing.Printer) || opts.Printing.Diagnose == nil {
		return nil
	}

	status, err := opts.Printing.Diagnose(wnd.Subject())
	if err != nil {
		// Wer den Druckerzustand nicht sehen darf, bekommt an dieser Stelle
		// keinen Hinweis – die Seite selbst bleibt nutzbar.
		return nil
	}

	problem := status.Problem()
	// Ein Rückstau im Druckdienst ist auch ohne Fehler eine Meldung wert: Er
	// erklärt, warum trotz "Fertig" in der Liste noch Papier kommt.
	backlog := spooledHint(status)

	if problem == "" && status.Message == "" && backlog == "" {
		return nil
	}

	// Ohne Problem ist die Gerätemeldung nur eine Information, kein Fehler.
	title := "Meldung des Druckers"
	if problem != "" {
		title = "Der Drucker ist nicht bereit"
	}

	return ui.VStack(
		ui.Text(title).Font(ui.TitleMedium),
		ui.If(problem != "", ui.Text(problem).Font(ui.BodyMedium).TextAlignment(ui.TextAlignCenter)),
		ui.If(status.Message != "", ui.Text(status.Message).Font(ui.MonoSmall).TextAlignment(ui.TextAlignCenter)),
		ui.If(backlog != "", ui.Text(backlog).Font(ui.BodySmall).TextAlignment(ui.TextAlignCenter)),
	).
		Gap(ui.L8).
		Alignment(ui.Center).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L24)).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.FullWidth())
}

// pendingBadge zeigt die Anzahl der noch offenen Aufträge.
func pendingBadge(pending int) core.View {
	if pending == 0 {
		return ui.Text("Warteschlange leer").Font(ui.LabelMedium)
	}

	label := "1 Auftrag offen"
	if pending > 1 {
		label = strconv.Itoa(pending) + " Aufträge offen"
	}

	return ui.Text(label).Font(ui.LabelMedium)
}

// spooledHint beschreibt den Rückstau in der Warteschlange des Druckdienstes.
//
// Diese Zahl stammt aus CUPS, nicht aus der Liste darunter. Weichen beide
// voneinander ab, liegen dort Aufträge, die die Fotobox nicht mehr verfolgt –
// und genau die kommen später unerwartet aus dem Gerät.
func spooledHint(status printing.PrinterStatus) string {
	n := len(status.Spooled)
	switch {
	case n == 0:
		return ""
	case n == 1:
		return "In der Warteschlange des Druckdienstes liegt noch 1 Auftrag (" + status.Spooled[0] + ")."
	default:
		return "In der Warteschlange des Druckdienstes liegen noch " + strconv.Itoa(n) + " Aufträge."
	}
}

// jobRow rendert eine Zeile der Auftragsliste.
func jobRow(wnd core.Window, opts Options, job printing.Job) core.View {
	return ui.HStack(
		stateDot(job.State),

		ui.VStack(
			ui.Text(printing.TemplateByID(job.Template).Name).Font(ui.LabelLarge),
			ui.Text(job.CreatedAt.Format("15:04:05")+" · "+job.Duration().String()).Font(ui.BodySmall),
			// Die Kennung der Warteschlange erlaubt es, denselben Auftrag im
			// Terminal wiederzufinden: lpstat -l -W completed -o <drucker>
			ui.If(job.PrinterJob != "", ui.Text(job.PrinterJob).Font(ui.MonoSmall)),
		).Gap(ui.L2).Alignment(ui.Leading),

		ui.Spacer(),

		ui.VStack(
			ui.Text(job.State.String()).Font(ui.LabelMedium),
			ui.If(job.Message != "", ui.Text(job.Message).Font(ui.BodySmall).
				TextAlignment(ui.TextAlignEnd)),
			// Der IPP-Grund ist nicht übersetzt und deshalb für die
			// Fehlersuche belastbarer als die Meldung. Bei erfolgreichen
			// Aufträgen trägt er nichts bei.
			ui.If(job.State == printing.StateFailed && job.Reason != "",
				ui.Text(job.Reason).Font(ui.MonoSmall).TextAlignment(ui.TextAlignEnd)),
		).Gap(ui.L2).Alignment(ui.Trailing),

		// Nur fehlgeschlagene Aufträge lassen sich wiederholen – typisch nach
		// leerem Papierfach.
		ui.If(job.State == printing.StateFailed, ui.SecondaryButton(func() {
			if err := opts.Printing.Retry(wnd.Subject(), job.ID); err != nil {
				alert.ShowBannerError(wnd, err)
				return
			}

			alert.ShowBannerMessage(wnd, alert.Message{
				Title:  "Erneut in der Warteschlange",
				Intent: alert.IntentOk,
			})
		}).Title("Wiederholen")),
	).
		Gap(ui.L16).
		Alignment(ui.Center).
		BackgroundColor(ui.M2).
		WithPadding(ui.Padding{}.All(ui.L16)).
		Border(ui.Border{}.Radius(ui.L12)).
		Frame(ui.Frame{}.FullWidth())
}

// stateDot signalisiert den Zustand farblich, damit die Liste aus einigen
// Metern Entfernung lesbar bleibt.
func stateDot(state printing.State) core.View {
	color := ui.M6
	switch state {
	case printing.StateQueued:
		color = ui.SW0
	case printing.StatePrinting:
		color = ui.SI0
	case printing.StateDone:
		color = ui.SG0
	case printing.StateFailed:
		color = ui.SE0
	}

	return ui.VStack().
		BackgroundColor(color).
		Border(ui.Border{}.Circle()).
		Frame(ui.Frame{}.Size(ui.L12, ui.L12))
}
