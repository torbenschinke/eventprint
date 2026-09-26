package camera

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

// TestGuardRestartsAfterPanic haelt fest, dass ein Absturz im Kamerapfad die
// Box nicht mitreisst.
//
// Ohne die Klammer riss eine Panik in einer der Goroutinen den gesamten
// Prozess mit: Galerie, Upload und Druckwarteschlange gingen fuer die Dauer
// des Neustarts mit unter. Auf einer Veranstaltung ohne Fernzugang ist das der
// Unterschied zwischen "die Kamera klemmt" und "die Box ist tot".
func TestGuardRestartsAfterPanic(t *testing.T) {
	shortGuardBackoff(t)

	var runs atomic.Int32
	st := &status{}

	guard(context.Background(), "test", st, func(context.Context) {
		if runs.Add(1) < 3 {
			panic("boom")
		}
		// Beim dritten Anlauf ordentlich zurueckkehren.
	})

	if got := runs.Load(); got != 3 {
		t.Fatalf("runs = %d, want 3 - die Schleife muss nach einem Absturz weiterlaufen", got)
	}
}

// TestGuardGivesUpAndReportsIt deckt die Dauerstoerung ab: Irgendwann ist
// Schluss, aber dann muss es sichtbar sein statt still.
func TestGuardGivesUpAndReportsIt(t *testing.T) {
	shortGuardBackoff(t)

	var runs atomic.Int32
	st := &status{}

	guard(context.Background(), "test", st, func(context.Context) {
		runs.Add(1)
		panic("dauerhaft kaputt")
	})

	if got := runs.Load(); got != guardLimit {
		t.Fatalf("runs = %d, want %d", got, guardLimit)
	}
	if got := st.get(); got.State != StateError || got.Detail == "" {
		t.Fatalf("status = %+v, want a visible error", got)
	}
}

// TestGuardStopsOnContext stellt sicher, dass ein Abbruch nicht als Absturz
// missverstanden wird und eine Neustartschleife ausloest.
func TestGuardStopsOnContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var runs atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		guard(ctx, "test", &status{}, func(c context.Context) {
			runs.Add(1)
			<-c.Done()
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("guard did not return after the context ended")
	}
	if got := runs.Load(); got > 1 {
		t.Fatalf("runs = %d, want at most 1", got)
	}
}

// TestRecoveryDoesNotBlockTethering ist der Fehler, den ich selbst eingebaut
// hatte.
//
// Die Nachlese laeuft vor dem Tethering, also in einer Zeit, in der niemand am
// USB lauscht. Ohne Deckel liefe sie auf einer vollen Karte minutenlang und
// erzeugte genau die Luecke, die sie schliessen soll.
func TestRecoveryDoesNotBlockTethering(t *testing.T) {
	runner := &blockingRunner{output: []byte("Canon EOS 80D usb:001,004\n")}
	s := newTestSupervisor(t.TempDir(), runner)
	s.recover = true
	s.recoverLimit = 200 * time.Millisecond

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.once(context.Background())
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("die Nachlese hat das Tethering blockiert")
	}

	if !runner.tethered.Load() {
		t.Fatal("nach der abgebrochenen Nachlese muss das Tethering trotzdem starten")
	}
}

// shortGuardBackoff kuerzt die Pause zwischen den Neustarts fuer den Test.
func shortGuardBackoff(t *testing.T) {
	t.Helper()
	previous := guardBackoff
	guardBackoff = time.Millisecond
	t.Cleanup(func() { guardBackoff = previous })
}

// blockingRunner laesst die Nachlese haengen, bis ihr Kontext endet, und
// merkt sich, ob danach noch getethert wurde.
type blockingRunner struct {
	output   []byte
	tethered atomic.Bool
}

func (r *blockingRunner) Output(context.Context, string, ...string) ([]byte, error) {
	return r.output, nil
}

func (r *blockingRunner) Run(ctx context.Context, _ io.Writer, _ string, args ...string) error {
	for _, arg := range args {
		if arg == "--capture-tethered" {
			r.tethered.Store(true)
			return errors.New("disconnected")
		}
	}

	// Die Nachlese: haengt, bis ihr eigenes Zeitlimit zuschlaegt.
	<-ctx.Done()
	return ctx.Err()
}
