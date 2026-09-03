package printing_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/worldiety/option"
	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

// fakePrinter ersetzt CUPS. Er nimmt jeden Auftrag an und meldet den zuvor
// festgelegten Ausgang – so lässt sich der Fall nachstellen, der uns die
// Ausdrucke gekostet hat: lp bestätigt, der Drucker verwirft.
type fakePrinter struct {
	outcome  printing.Outcome
	printErr error
	printed  int

	// canceled hält fest, welche Aufträge zurückgenommen wurden. Genau daran
	// entscheidet sich, ob eine Wiederholung ein zweites Blatt erzeugt.
	canceled []string

	// status ist der Zustand, den der Drucker meldet. Der Nullwert bedeutet
	// "bereit".
	status *printing.PrinterStatus
}

func (p *fakePrinter) Name() string { return "Fake" }

func (p *fakePrinter) Print(context.Context, []byte, string) (printing.Result, error) {
	if p.printErr != nil {
		return printing.Result{}, p.printErr
	}

	p.printed++

	return printing.Result{JobID: "Fake-1", Message: "angenommen"}, nil
}

func (p *fakePrinter) Await(context.Context, string) printing.Outcome { return p.outcome }

func (p *fakePrinter) Status(context.Context) printing.PrinterStatus {
	if p.status != nil {
		return *p.status
	}

	return printing.PrinterStatus{Queue: "Fake", Exists: true, Enabled: true, Accepting: true}
}

func (p *fakePrinter) Cancel(_ context.Context, jobID string) error {
	p.canceled = append(p.canceled, jobID)

	return nil
}

// openSample liefert das Beispielbild als Originaldaten des Fotos.
func openSample(t *testing.T) photo.OpenOriginal {
	buf := loadSample(t)

	return func(auth.Subject, photo.ID) (option.Opt[io.ReadCloser], error) {
		return option.Some[io.ReadCloser](io.NopCloser(bytes.NewReader(buf))), nil
	}
}

func newTestUseCases(t *testing.T, printer printing.Printer) printing.UseCases {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Ein Repository über einen Speicher-Blobstore: dieselbe Serialisierung
	// wie im Betrieb, nur ohne Datei auf der Platte.
	repo := json.NewSloppyJSONRepository[printing.Job, printing.JobID](mem.NewBlobStore("printjob"))

	return printing.NewUseCases(ctx, events.NewEventBus(), repo, printer, openSample(t), nil)
}

// awaitJob wartet, bis der Auftrag abgeschlossen ist.
func awaitJob(t *testing.T, uc printing.UseCases, id printing.JobID) printing.Job {
	t.Helper()

	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		opt, err := uc.FindJobByID(user.SU(), id)
		if err != nil {
			t.Fatalf("FindJobByID: %v", err)
		}

		if opt.IsSome() && opt.Unwrap().State.Done() {
			return opt.Unwrap()
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Fatal("der Auftrag wurde nicht abgeschlossen")

	return printing.Job{}
}

// TestWorkerReportsRejectionByPrinter ist der Kern dieser Absicherung.
//
// Vorher galt ein Auftrag als "Fertig", sobald lp ihn angenommen hatte. Ein
// vom Backend verworfener Auftrag – etwa weil CUPS den Dateityp nicht
// erkennt – erschien dadurch als Erfolg, während nichts gedruckt wurde.
func TestWorkerReportsRejectionByPrinter(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{
		Done:    true,
		Success: false,
		Reason:  "canceled-at-device",
		Message: "The print file could not be opened.",
	}}

	uc := newTestUseCases(t, printer)

	id, err := uc.Print(user.SU(), "irgendein-foto", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	job := awaitJob(t, uc, id)

	if job.State != printing.StateFailed {
		t.Errorf("Zustand = %s, erwartet %s – ein verworfener Auftrag darf nicht als Erfolg gelten",
			job.State, printing.StateFailed)
	}

	if job.Reason != "canceled-at-device" {
		t.Errorf("Reason = %q, erwartet %q", job.Reason, "canceled-at-device")
	}

	if job.Message != "The print file could not be opened." {
		t.Errorf("Message = %q – die Ursache von CUPS muss durchgereicht werden", job.Message)
	}

	if job.PrinterJob != "Fake-1" {
		t.Errorf("PrinterJob = %q, erwartet %q – ohne Kennung ist der Auftrag nicht auffindbar", job.PrinterJob, "Fake-1")
	}
}

// TestWorkerReportsSuccess ist die Gegenprobe.
func TestWorkerReportsSuccess(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{
		Done:    true,
		Success: true,
		Reason:  "job-completed-successfully",
	}}

	uc := newTestUseCases(t, printer)

	id, err := uc.Print(user.SU(), "irgendein-foto", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	job := awaitJob(t, uc, id)

	if job.State != printing.StateDone {
		t.Errorf("Zustand = %s, erwartet %s (%s)", job.State, printing.StateDone, job.Message)
	}

	if printer.printed != 1 {
		t.Errorf("es wurden %d Aufträge übergeben, erwartet 1", printer.printed)
	}

	spec.Verified(t, druck.RDruckAuftrag)
}

// TestWorkerReportsSubmissionFailure deckt den Fall ab, dass schon die
// Übergabe scheitert – etwa weil lp fehlt.
func TestWorkerReportsSubmissionFailure(t *testing.T) {
	printer := &fakePrinter{printErr: errors.New("lp failed: exec: \"lp\": executable file not found in $PATH")}

	uc := newTestUseCases(t, printer)

	id, err := uc.Print(user.SU(), "irgendein-foto", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	job := awaitJob(t, uc, id)

	if job.State != printing.StateFailed {
		t.Errorf("Zustand = %s, erwartet %s", job.State, printing.StateFailed)
	}

	if job.Message == "" {
		t.Error("die Ursache muss an der Oberfläche ankommen")
	}
}

// TestRetryRequeuesFailedJob prüft den Ablauf nach einem Papierwechsel.
func TestRetryRequeuesFailedJob(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Reason: "canceled-at-device"}}

	uc := newTestUseCases(t, printer)

	id, err := uc.Print(user.SU(), "irgendein-foto", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	awaitJob(t, uc, id)

	// Papier nachgelegt: ab jetzt gelingt der Druck.
	printer.outcome = printing.Outcome{Done: true, Success: true, Reason: "job-completed-successfully"}

	if err := uc.Retry(user.SU(), id); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	job := awaitJob(t, uc, id)

	if job.State != printing.StateDone {
		t.Errorf("Zustand nach Wiederholung = %s, erwartet %s", job.State, printing.StateDone)
	}

	if printer.printed != 2 {
		t.Errorf("es wurden %d Aufträge übergeben, erwartet 2", printer.printed)
	}
}

// TestRetryCancelsPreviousPrinterJob sichert die zweite Hälfte des
// Doppeldrucks ab.
//
// Ein fehlgeschlagener Auftrag kann im Druckdienst weiterhin anhängig sein –
// nach einem Timeout ist das sogar der Regelfall. Wird er vor der
// Wiederholung nicht zurückgenommen, existieren zwei Aufträge für dasselbe
// Bild, und der Drucker gibt es zweimal aus.
func TestRetryCancelsPreviousPrinterJob(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Reason: "timeout"}}

	uc := newTestUseCases(t, printer)

	id, err := uc.Print(user.SU(), "irgendein-foto", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	awaitJob(t, uc, id)

	printer.outcome = printing.Outcome{Done: true, Success: true, Reason: "job-completed-successfully"}

	if err := uc.Retry(user.SU(), id); err != nil {
		t.Fatalf("Retry: %v", err)
	}

	awaitJob(t, uc, id)

	if len(printer.canceled) != 1 || printer.canceled[0] != "Fake-1" {
		t.Fatalf("stornierte Aufträge = %v, erwartet [Fake-1]", printer.canceled)
	}

	spec.Verified(t, druck.RDruckWiederholung)
}

// TestJobsAreListedNewestFirst deckt den Zustand der Warteschlange ab, wie ihn
// die Druckstatus-Seite zeigt.
func TestJobsAreListedNewestFirst(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Success: true, Reason: "job-completed-successfully"}}

	uc := newTestUseCases(t, printer)

	first, err := uc.Print(user.SU(), "foto-1", printing.TemplateFull)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	awaitJob(t, uc, first)

	second, err := uc.Print(user.SU(), "foto-2", printing.TemplatePassepartout)
	if err != nil {
		t.Fatalf("Print: %v", err)
	}

	awaitJob(t, uc, second)

	var ids []printing.JobID
	for job, err := range uc.FindAllJobs(user.SU()) {
		if err != nil {
			t.Fatalf("FindAllJobs: %v", err)
		}

		ids = append(ids, job.ID)
	}

	if len(ids) != 2 {
		t.Fatalf("FindAllJobs lieferte %d Aufträge, erwartet 2", len(ids))
	}

	if ids[0] != second || ids[1] != first {
		t.Fatal("die Aufträge stehen nicht mit dem neuesten zuerst")
	}

	// Der einzelne Auftrag trägt den Grund, den die Oberfläche anzeigt.
	opt, err := uc.FindJobByID(user.SU(), first)
	if err != nil {
		t.Fatalf("FindJobByID: %v", err)
	}

	if opt.IsNone() {
		t.Fatal("der eben gedruckte Auftrag wurde nicht gefunden")
	}

	if got := opt.Unwrap(); got.State != printing.StateDone || got.Reason != "job-completed-successfully" {
		t.Fatalf("Zustand %q, Grund %q – erwartet fertig mit IPP-Grund", got.State, got.Reason)
	}

	missing, err := uc.FindJobByID(user.SU(), "gibt-es-nicht")
	if err != nil {
		t.Fatalf("FindJobByID(unbekannt): %v", err)
	}

	if missing.IsSome() {
		t.Fatal("eine unbekannte Kennung lieferte einen Auftrag")
	}

	spec.Verified(t, druck.RDruckStatus)
}

// TestPreviewRendersWithoutPrinting sichert zu, dass die Vorschau das Ergebnis
// zeigt, ohne Papier zu verbrauchen.
func TestPreviewRendersWithoutPrinting(t *testing.T) {
	printer := &fakePrinter{outcome: printing.Outcome{Done: true, Success: true}}

	uc := newTestUseCases(t, printer)

	seen := map[string]bool{}

	for _, tpl := range printing.Templates() {
		buf, err := uc.Preview(user.SU(), "irgendein-foto", tpl.ID)
		if err != nil {
			t.Fatalf("Preview(%s): %v", tpl.ID, err)
		}

		if len(buf) == 0 || !bytes.HasPrefix(buf, []byte{0xFF, 0xD8}) {
			t.Fatalf("Preview(%s) lieferte kein JPEG (%d Bytes)", tpl.ID, len(buf))
		}

		// Jedes Layout muss ein anderes Bild ergeben, sonst zeigt die
		// Vorschau dreimal dasselbe und die Auswahl wäre eine Behauptung.
		key := string(buf)
		if seen[key] {
			t.Fatalf("Layout %s liefert dieselbe Vorschau wie ein anderes", tpl.ID)
		}

		seen[key] = true
	}

	if printer.printed != 0 {
		t.Fatalf("die Vorschau hat %d Aufträge an den Drucker übergeben, erwartet 0", printer.printed)
	}

	spec.Verified(t, druck.RDruckVorschau)
}

// TestDiagnoseReportsPrinterState prüft die Auskunft, die der Betreuung das
// Terminal ersparen soll.
func TestDiagnoseReportsPrinterState(t *testing.T) {
	printer := &fakePrinter{}

	uc := newTestUseCases(t, printer)

	if status := uc.Diagnose(user.SU()); !status.OK() || status.Problem() != "" {
		t.Fatalf("ein bereiter Drucker meldet ein Problem: %q", status.Problem())
	}

	// Angehaltener Drucker mit Gerätemeldung.
	printer.status = &printing.PrinterStatus{
		Queue: "Fake", Exists: true, Enabled: false, Accepting: true,
		Message: "Out of paper",
	}

	status := uc.Diagnose(user.SU())

	if status.OK() {
		t.Fatal("ein angehaltener Drucker gilt als bereit")
	}

	if status.Problem() == "" {
		t.Fatal("zum angehaltenen Drucker fehlt die Erklärung im Klartext")
	}

	if status.Message != "Out of paper" {
		t.Fatalf("die Meldung des Geräts fehlt: %q", status.Message)
	}

	// Fehlende Warteschlange ist der zweite Fall, der sonst nur im Terminal
	// sichtbar wäre.
	printer.status = &printing.PrinterStatus{Queue: "Fake", Exists: false}

	if status := uc.Diagnose(user.SU()); status.OK() || status.Problem() == "" {
		t.Fatal("eine fehlende Warteschlange wird nicht als Problem gemeldet")
	}

	spec.Verified(t, druck.RDruckDiagnose)
}
