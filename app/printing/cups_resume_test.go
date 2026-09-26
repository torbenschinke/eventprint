package printing

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/worldiety/speclink/spec"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/requirements/fun/druck"
)

// stoppedPrinter ist ein Drucker, den CUPS angehalten hat. Jede Freigabe
// wird gezählt; stubborn bildet das leere Farbband nach, bei dem CUPS die
// Warteschlange nach jedem Versuch sofort wieder anhält.
type stoppedPrinter struct {
	mutex    sync.Mutex
	enabled  bool
	stubborn bool
	resumed  int
}

func (p *stoppedPrinter) Name() string { return "CZ01" }

func (p *stoppedPrinter) Print(context.Context, []byte, string) (Result, error) {
	return Result{}, nil
}

func (p *stoppedPrinter) Await(context.Context, string) Outcome { return Outcome{Done: true} }

func (p *stoppedPrinter) Status(context.Context) PrinterStatus {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return PrinterStatus{
		Queue: "CZ01", Exists: true, Enabled: p.enabled, Accepting: true,
		Message: "Printer not ready: Ribbon End, please correct...",
	}
}

func (p *stoppedPrinter) Resume(context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.resumed++
	p.enabled = !p.stubborn

	return nil
}

func (p *stoppedPrinter) resumes() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.resumed
}

func runGuard(t *testing.T, printer Printer) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	g := &resumeGuard{printer: printer, interval: 5 * time.Millisecond, first: 20 * time.Millisecond, max: 80 * time.Millisecond}
	go g.run(ctx)
}

// TestResumeGuardReleasesStoppedQueue ist der Abend vom 05.09.2026: Nach dem
// Papierwechsel stand die Warteschlange, bis jemand im Terminal cupsenable
// eingab. Jetzt gibt die Fotobox sie selbst frei – genau einmal.
func TestResumeGuardReleasesStoppedQueue(t *testing.T) {
	printer := &stoppedPrinter{}
	runGuard(t, printer)

	deadline := time.Now().Add(5 * time.Second)
	for printer.resumes() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	if printer.resumes() == 0 {
		t.Fatal("der angehaltene Drucker wurde nicht freigegeben")
	}

	time.Sleep(200 * time.Millisecond)

	if n := printer.resumes(); n != 1 {
		t.Fatalf("ein wieder laufender Drucker wurde erneut freigegeben: %d Versuche", n)
	}

	spec.Verified(t, druck.RDruckFreigabe)
}

// TestResumeGuardBacksOff sichert ab, dass ein dauerhaft leeres Farbband
// nicht im Takt der Abfrage freigegeben wird.
func TestResumeGuardBacksOff(t *testing.T) {
	printer := &stoppedPrinter{stubborn: true}
	runGuard(t, printer)

	time.Sleep(400 * time.Millisecond)

	// Ohne wachsenden Abstand wären es bei 5 ms Abfragetakt rund 80.
	n := printer.resumes()
	if n == 0 {
		t.Fatal("der angehaltene Drucker wurde nie freigegeben")
	}

	if n > 10 {
		t.Fatalf("zu viele Freigaben ohne wachsenden Abstand: %d", n)
	}
}

// TestResumeGuardLeavesMissingQueueAlone: Eine fehlende Warteschlange lässt
// sich nicht freigeben, cupsenable würde nur das Protokoll füllen.
func TestResumeGuardLeavesMissingQueueAlone(t *testing.T) {
	printer := &missingPrinter{}
	runGuard(t, printer)

	time.Sleep(100 * time.Millisecond)

	if printer.resumes() != 0 {
		t.Fatal("eine fehlende Warteschlange wurde freigegeben")
	}
}

type missingPrinter struct{ stoppedPrinter }

func (p *missingPrinter) Status(context.Context) PrinterStatus {
	return PrinterStatus{Queue: "CZ01"}
}

// TestResumeUseCase ist die Schaltfläche "Weiter drucken".
func TestResumeUseCase(t *testing.T) {
	printer := &stoppedPrinter{}

	if err := NewResume(context.Background(), printer)(user.SU()); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	if printer.resumes() != 1 || !printer.Status(context.Background()).Enabled {
		t.Fatal("der Drucker wurde nicht freigegeben")
	}

	spec.Verified(t, druck.RDruckFreigabe)
}

// TestResumeQueueCallsCupsenable hält den Aufruf fest, der vorher von Hand
// im Terminal nötig war.
func TestResumeQueueCallsCupsenable(t *testing.T) {
	dir := t.TempDir()
	args := filepath.Join(dir, "args")
	script := filepath.Join(dir, "cupsenable")

	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+args+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := cupsenableExecutable
	cupsenableExecutable = script
	t.Cleanup(func() { cupsenableExecutable = old })

	if err := ResumeQueue(context.Background(), "CZ01"); err != nil {
		t.Fatalf("ResumeQueue: %v", err)
	}

	got, err := os.ReadFile(args)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(got)) != "CZ01" {
		t.Fatalf("cupsenable mit falschen Argumenten aufgerufen: %q", got)
	}
}

// TestProblemNamesTheCause: Die Meldung des Backends ist englisch und
// technisch. Die Betreuung soll lesen, was zu tun ist.
func TestProblemNamesTheCause(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"Printer not ready: Ribbon End, please correct...", "Papier oder Farbband ist leer."},
		{"Fatal Printer Error: 9999 => Communication Failure, halting queue!", "USB-Verbindung"},
		{"Failure to send data to printer (libusb error -1: (0/32 to 0x01))", "USB-Verbindung"},
	}

	for _, tt := range tests {
		status := PrinterStatus{Queue: "CZ01", Exists: true, Accepting: true, Message: tt.message}

		if !strings.Contains(status.Problem(), tt.want) {
			t.Errorf("%q: Problem() = %q, erwartet %q", tt.message, status.Problem(), tt.want)
		}
	}
}

// stubQueue ersetzt lpstat durch einen Druckdienst, der den Auftrag CZ01-40
// als wartend führt und den Drucker je nach stopped als angehalten meldet.
func stubQueue(t *testing.T, stopped bool) {
	t.Helper()

	printer := "printer CZ01 is idle.  enabled since Sat 05 Sep 2026"
	if stopped {
		printer = "printer CZ01 disabled since Sat 05 Sep 2026 -\n\tPrinter not ready: Ribbon End, please correct..."
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "lpstat")
	body := "#!/bin/sh\n" +
		"case \"$1 $2\" in\n" +
		"  \"-p \"*) printf '%s\\n' '" + strings.ReplaceAll(printer, "\n", "'\"\\n\"'") + "' ;;\n" +
		"  \"-W not-completed\") echo 'CZ01-40 eventprint 1024 Sat 05 Sep 2026' ;;\n" +
		"esac\n"

	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	old := lpstatExecutable
	lpstatExecutable = script
	t.Cleanup(func() { lpstatExecutable = old })

	cancelScript := filepath.Join(dir, "cancel")
	if err := os.WriteFile(cancelScript, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	oldCancel := cancelExecutable
	cancelExecutable = cancelScript
	t.Cleanup(func() { cancelExecutable = oldCancel })
}

// TestAwaitJobPausesDeadlineWhileStopped: Während des Papierwechsels darf die
// Frist nicht ablaufen, sonst storniert die Fotobox genau den Auftrag, der
// nach dem Wechsel gedruckt würde.
func TestAwaitJobPausesDeadlineWhileStopped(t *testing.T) {
	stubQueue(t, true)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	outcome := AwaitJob(ctx, "CZ01", "CZ01-40", 20*time.Millisecond, 5*time.Millisecond)

	if outcome.Reason != "canceled" {
		t.Fatalf("Reason = %q: die Frist lief trotz angehaltenem Drucker ab", outcome.Reason)
	}

	spec.Verified(t, druck.RDruckFreigabe)
}

// TestAwaitJobKeepsDeadlineWhileRunning ist die Gegenprobe: Ein laufender
// Drucker, der einen Auftrag nicht abschließt, hängt – dort gilt die Frist.
func TestAwaitJobKeepsDeadlineWhileRunning(t *testing.T) {
	stubQueue(t, false)

	outcome := AwaitJob(context.Background(), "CZ01", "CZ01-40", 20*time.Millisecond, 5*time.Millisecond)

	if outcome.Reason != "timeout" {
		t.Fatalf("Reason = %q, erwartet timeout", outcome.Reason)
	}
}
