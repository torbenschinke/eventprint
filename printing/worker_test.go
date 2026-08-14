package printing_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/blob/mem"
	"go.wdy.de/nago/pkg/data/json"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
)

// fakePrinter ersetzt CUPS. Er nimmt jeden Auftrag an und meldet den zuvor
// festgelegten Ausgang – so lässt sich der Fall nachstellen, der uns die
// Ausdrucke gekostet hat: lp bestätigt, der Drucker verwirft.
type fakePrinter struct {
	outcome  printing.Outcome
	printErr error
	printed  int
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
	return printing.PrinterStatus{Queue: "Fake", Exists: true, Enabled: true, Accepting: true}
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

	return printing.NewUseCases(ctx, events.NewEventBus(), repo, printer, openSample(t), printing.Options{})
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
