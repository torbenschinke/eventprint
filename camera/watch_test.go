package camera

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"testing"

	nagoimage "go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
)

func TestDetectedCamera(t *testing.T) {
	model, port, ok := detectedCamera("Modell                         Port\n----------------------------------------------------------\nCanon EOS 80D                  usb:001,004\n")
	if !ok || model != "Canon EOS 80D" || port != "usb:001,004" {
		t.Fatalf("detectedCamera = %q, %q, %v", model, port, ok)
	}
	if _, _, ok := detectedCamera("Modell Port\n------ ----\n"); ok {
		t.Fatal("camera detected in empty output")
	}
}

func TestSupervisorStartsTetheringForDetectedCamera(t *testing.T) {
	runner := &recordingRunner{output: []byte("Modell Port\n------ ----\nCanon EOS 80D usb:001,004\n"), runErr: errors.New("disconnected")}
	if !superviseOnce(context.Background(), t.TempDir(), runner) {
		t.Fatal("detected camera was not started")
	}
	if runner.name != "gphoto2" {
		t.Fatalf("command = %q", runner.name)
	}
	want := []string{"--camera", "Canon EOS 80D", "--port", "usb:001,004", "--capture-tethered", "--filename"}
	for i, arg := range want {
		if runner.args[i] != arg {
			t.Fatalf("arg %d = %q, want %q", i, runner.args[i], arg)
		}
	}
	if len(runner.args) != len(want)+1 || filepath.Ext(runner.args[len(runner.args)-1]) != ".%C" {
		t.Fatalf("unexpected filename pattern %q", runner.args[len(runner.args)-1])
	}
}

func TestWatcherImportsAndPrintsStableCaptureAsPolaroid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	if err := os.WriteFile(path, testJPEG(t), 0o600); err != nil {
		t.Fatal(err)
	}

	var imported photo.Options
	var printed printing.TemplateID
	w := newTestWatcher(dir, func() Options { return Options{AutoPrint: true} }, &imported, &printed)
	w.scan()
	if imported.Source != "" {
		t.Fatal("growing capture imported on first scan")
	}
	w.scan()
	if imported.Source != photo.SourceCamera {
		t.Fatalf("source = %q, want camera", imported.Source)
	}
	if printed != printing.TemplatePolaroid {
		t.Fatalf("template = %q, want polaroid", printed)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("imported file remains: %v", err)
	}
}

func TestWatcherKeepsCaptureInHistoryWithoutAutoPrint(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	if err := os.WriteFile(path, testJPEG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	var imported photo.Options
	var printed printing.TemplateID
	w := newTestWatcher(dir, func() Options { return Options{AutoPrint: false, AutoPrintTemplate: printing.TemplateBorder} }, &imported, &printed)
	w.scan()
	w.scan()
	if imported.Source != photo.SourceCamera {
		t.Fatalf("source = %q, want camera", imported.Source)
	}
	if printed != "" {
		t.Fatalf("unexpected print with template %q", printed)
	}
}

func TestWatcherRetriesPrintWithoutDuplicateImport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture.jpg")
	if err := os.WriteFile(path, testJPEG(t), 0o600); err != nil {
		t.Fatal(err)
	}
	imports, attempts := 0, 0
	w := &watcher{
		dir: dir, load: func() Options { return Options{AutoPrint: true, AutoPrintTemplate: printing.TemplateBorder} },
		photos: photo.UseCases{Import: func(_ user.Subject, opts photo.Options, _ nagoimage.File) (photo.Photo, error) {
			imports++
			return photo.Photo{ID: "photo"}, nil
		}},
		prints: printing.UseCases{Print: func(_ user.Subject, _ photo.ID, _ printing.TemplateID) (printing.JobID, error) {
			attempts++
			if attempts == 1 {
				return "", errors.New("printer unavailable")
			}
			return "job", nil
		}},
		states: map[string]fileState{}, pending: map[string]pendingPhoto{},
	}
	w.scan()
	w.scan()
	w.scan()
	if imports != 1 || attempts != 2 {
		t.Fatalf("imports=%d attempts=%d, want 1/2", imports, attempts)
	}
}

func newTestWatcher(dir string, load LoadOptions, imported *photo.Options, printed *printing.TemplateID) *watcher {
	return &watcher{
		dir: dir, load: load,
		photos: photo.UseCases{Import: func(_ user.Subject, opts photo.Options, _ nagoimage.File) (photo.Photo, error) {
			*imported = opts
			return photo.Photo{ID: "photo"}, nil
		}},
		prints: printing.UseCases{Print: func(_ user.Subject, _ photo.ID, tpl printing.TemplateID) (printing.JobID, error) {
			*printed = tpl
			return "job", nil
		}},
		states: map[string]fileState{}, pending: map[string]pendingPhoto{},
	}
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.Black)
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, nil); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

type recordingRunner struct {
	output []byte
	runErr error
	name   string
	args   []string
}

func (r *recordingRunner) Output(context.Context, string, ...string) ([]byte, error) {
	return r.output, nil
}

func (r *recordingRunner) Run(_ context.Context, _ io.Writer, name string, args ...string) error {
	r.name = name
	r.args = append([]string(nil), args...)
	return r.runErr
}
