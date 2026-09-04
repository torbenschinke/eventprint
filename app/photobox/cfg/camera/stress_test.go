package camera

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	nagoimage "go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// TestMonitorUnderBurst schuettelt den nebenlaeufigen Aufbau durch.
//
// Der Race-Detector laeuft weder auf der Entwicklungsmaschine noch auf dem
// Geraet - beide haben eine zu kleine VMA-Breite fuer den ThreadSanitizer.
// Dieser Test ersetzt ihn nicht, er faengt aber das ab, was ohne Kamera sonst
// niemand bemerkt haette: Verklemmungen, verlorene Aufnahmen und Abstuerze
// unter Serienausloesung.
func TestMonitorUnderBurst(t *testing.T) {
	const burst = 60

	dir := t.TempDir()
	var printed atomic.Int32
	seen := &sync.Map{}

	m := New(dir, func() Options {
		return Options{AutoPrint: true, ScanInterval: 5 * time.Millisecond, DetectInterval: time.Minute}
	}, photo.UseCases{
		Import: func(_ user.Subject, _ photo.Options, f nagoimage.File) (photo.Photo, error) {
			mem := f.(nagoimage.MemFile)
			if _, dup := seen.LoadOrStore(mem.Filename, true); dup {
				t.Errorf("Aufnahme %s wurde doppelt importiert", mem.Filename)
			}
			return photo.Photo{ID: photo.ID(mem.Filename)}, nil
		},
	}, printing.UseCases{
		Print: func(_ user.Subject, _ photo.ID, _ printing.TemplateID) (printing.JobID, error) {
			printed.Add(1)
			return "job", nil
		},
	})
	m.runner = idleRunner{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)
	time.Sleep(100 * time.Millisecond)

	// Serienausloesung: schneller, als der Worker abarbeiten kann.
	jpeg := testJPEG(t)
	for i := range burst {
		path := filepath.Join(dir, fmt.Sprintf("capture-%03d.jpg", i))
		if err := os.WriteFile(path, jpeg, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if printed.Load() == burst {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := printed.Load(); got != burst {
		t.Fatalf("gedruckt = %d, want %d - unter Serienausloesung gingen Aufnahmen verloren", got, burst)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("%d Dateien blieben liegen, want 0", len(entries))
	}

	status := m.Status()
	if status.Captures != burst {
		t.Fatalf("Status meldet %d Aufnahmen, want %d", status.Captures, burst)
	}
}

// TestMonitorSurvivesFlappingCamera stellt eine Kamera nach, die dauernd die
// Verbindung verliert - der Fall, der die erste Veranstaltung gekostet hat.
func TestMonitorSurvivesFlappingCamera(t *testing.T) {
	dir := t.TempDir()
	runner := &flappingRunner{}

	m := New(dir, func() Options {
		return Options{ScanInterval: 10 * time.Millisecond, DetectInterval: time.Millisecond}
	}, photo.UseCases{}, printing.UseCases{})
	m.runner = runner
	m.healthy = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	waitFor(t, func() bool { return m.Status().Drops >= 3 }, "die Abrisse wurden nicht gezaehlt")

	// Nach den Abrissen muss die Nachlese angelaufen sein.
	waitFor(t, func() bool { return runner.recoveries.Load() > 0 },
		"nach einem Abriss wurde die Speicherkarte nie nachgelesen")

	if m.Status().State == "" {
		t.Fatal("der Status blieb leer")
	}
}

// flappingRunner liefert eine Kamera, deren Tethering staendig abreisst.
type flappingRunner struct {
	recoveries atomic.Int32
}

func (r *flappingRunner) Output(context.Context, string, ...string) ([]byte, error) {
	return []byte("Canon EOS 80D usb:001,004\n"), nil
}

func (r *flappingRunner) Run(ctx context.Context, _ io.Writer, _ string, args ...string) error {
	for _, arg := range args {
		switch arg {
		case "--get-all-files":
			r.recoveries.Add(1)
			return nil
		case "--capture-tethered":
			// Lange genug, um als geglueckt zu gelten, dann abreissen.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(30 * time.Millisecond):
			}
			return errors.New("ptp: device disconnected")
		}
	}
	return nil
}
