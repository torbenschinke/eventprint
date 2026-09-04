package camera

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	nagoimage "go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// TestMonitorDeliversCaptureEndToEnd laesst den echten, nebenlaeufigen Aufbau
// laufen: Supervisor, Verzeichnisueberwachung und Worker.
//
// Die Einzeltests fassen die Bausteine nacheinander an. Dieser hier prueft,
// dass sie auch zusammen funktionieren - und dass eine Aufnahme schnell
// ankommt, statt auf mehrere Takte des Durchlaufs zu warten.
func TestMonitorDeliversCaptureEndToEnd(t *testing.T) {
	dir := t.TempDir()

	var mu sync.Mutex
	done := make(chan printing.TemplateID, 1)

	m := New(dir, func() Options {
		// Der Rueckfall-Takt steht bewusst hoch: Kommt das Bild trotzdem
		// zuegig an, kam die Meldung vom Kernel und nicht vom Warten.
		return Options{AutoPrint: true, ScanInterval: time.Minute, DetectInterval: time.Minute}
	}, photo.UseCases{
		Import: func(_ user.Subject, _ photo.Options, _ nagoimage.File) (photo.Photo, error) {
			mu.Lock()
			defer mu.Unlock()
			return photo.Photo{ID: "photo"}, nil
		},
	}, printing.UseCases{
		Print: func(_ user.Subject, _ photo.ID, tpl printing.TemplateID) (printing.JobID, error) {
			select {
			case done <- tpl:
			default:
			}
			return "job", nil
		},
	})
	m.runner = idleRunner{}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	// Erst schreiben, wenn die Ueberwachung steht, sonst faengt sie der
	// Erstdurchlauf ab und die Aussage waere keine.
	time.Sleep(150 * time.Millisecond)
	path := filepath.Join(dir, "capture.jpg")
	writeFile(t, path, testJPEG(t))

	select {
	case tpl := <-done:
		if tpl != printing.TemplatePolaroid {
			t.Fatalf("template = %q, want polaroid", tpl)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("capture never reached the printer")
	}

	waitFor(t, func() bool {
		_, err := os.Stat(path)
		return os.IsNotExist(err)
	}, "imported file was never removed")

	waitFor(t, func() bool { return m.Status().Captures == 1 }, "status never reported the capture")
}

// TestMonitorReportsMissingCamera stellt sicher, dass der Startbildschirm die
// fehlende Kamera meldet, statt stumm auszusehen wie im Normalbetrieb.
func TestMonitorReportsMissingCamera(t *testing.T) {
	m := New(t.TempDir(), func() Options {
		return Options{ScanInterval: time.Minute, DetectInterval: 10 * time.Millisecond}
	}, photo.UseCases{}, printing.UseCases{})
	m.runner = idleRunner{detectErr: errors.New("gphoto2: not found")}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go m.Run(ctx)

	waitFor(t, func() bool { return m.Status().State == StateError }, "missing gphoto2 was never reported")
}

func waitFor(t *testing.T, ok func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

// idleRunner spielt eine Umgebung ohne Kamera nach, ohne den Bus zu belegen.
type idleRunner struct {
	detectErr error
}

func (r idleRunner) Output(context.Context, string, ...string) ([]byte, error) {
	return []byte("Modell Port\n------ ----\n"), r.detectErr
}

func (idleRunner) Run(ctx context.Context, _ io.Writer, _ string, _ ...string) error {
	<-ctx.Done()
	return ctx.Err()
}
