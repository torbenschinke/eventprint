// Package camera detects USB/PTP cameras through gphoto2 and imports tethered captures.
package camera

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/photo"
	"github.com/torbenschinke/eventprint/printing"
)

type Options struct {
	AutoPrint         bool
	AutoPrintTemplate printing.TemplateID
	ScanInterval      time.Duration
	DetectInterval    time.Duration
}

type LoadOptions func() Options

type commandRunner interface {
	Output(context.Context, string, ...string) ([]byte, error)
	Run(context.Context, io.Writer, string, ...string) error
}

type execRunner struct{}

func (execRunner) Output(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).Output()
}

func (execRunner) Run(ctx context.Context, output io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

type fileState struct {
	size    int64
	modTime time.Time
}

type pendingPhoto struct {
	id       photo.ID
	template printing.TemplateID

	// printed hält fest, dass der Auftrag bereits eingereiht wurde.
	//
	// Der Eintrag bleibt bestehen, bis die Datei gelöscht ist. Scheitert das
	// Löschen – etwa weil das Verzeichnis schreibgeschützt ist –, sah der
	// nächste Durchlauf im Sekundentakt einen offenen Eintrag und druckte
	// erneut. Ohne diese Markierung wäre eine einzige klemmende Datei ein
	// endloser Papierverbrauch.
	printed bool
}

// Run keeps camera detection, tethering, importing and printing alive until ctx ends.
func Run(ctx context.Context, dir string, load LoadOptions, photos photo.UseCases, prints printing.UseCases) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		slog.Error("cannot create camera directory", "dir", dir, "err", err)
		return
	}

	w := &watcher{
		dir:     dir,
		load:    load,
		photos:  photos,
		prints:  prints,
		states:  map[string]fileState{},
		pending: map[string]pendingPhoto{},
	}
	go supervise(ctx, dir, execRunner{}, load)
	w.run(ctx)
}

func supervise(ctx context.Context, dir string, runner commandRunner, load LoadOptions) {
	for {
		if ctx.Err() != nil {
			return
		}

		superviseOnce(ctx, dir, runner)

		interval := load().DetectInterval
		if interval <= 0 {
			interval = 10 * time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func superviseOnce(ctx context.Context, dir string, runner commandRunner) bool {
	out, err := runner.Output(ctx, "gphoto2", "--auto-detect")
	if err != nil {
		slog.Error("cannot detect camera with gphoto2", "err", err)
		return false
	}
	model, port, ok := detectedCamera(string(out))
	if !ok {
		return false
	}
	slog.Info("camera connected", "model", model, "port", port)
	prefix := fmt.Sprintf("capture-%d-%%Y%%m%%d-%%H%%M%%S-%%03n.%%C", time.Now().UnixNano())
	args := []string{"--camera", model, "--port", port, "--capture-tethered", "--filename", filepath.Join(dir, prefix)}
	if err := runner.Run(ctx, os.Stderr, "gphoto2", args...); err != nil && ctx.Err() == nil {
		slog.Warn("camera disconnected or tethering stopped", "model", model, "err", err)
	}
	return true
}

func detectedCamera(output string) (model, port string, ok bool) {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.HasPrefix(fields[len(fields)-1], "usb:") {
			continue
		}
		return strings.Join(fields[:len(fields)-1], " "), fields[len(fields)-1], true
	}
	return "", "", false
}

type watcher struct {
	dir    string
	load   LoadOptions
	photos photo.UseCases
	prints printing.UseCases

	states  map[string]fileState
	pending map[string]pendingPhoto
}

func (w *watcher) run(ctx context.Context) {
	w.scan()
	interval := w.load().ScanInterval
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

func (w *watcher) scan() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		slog.Error("cannot read camera directory", "dir", w.dir, "err", err)
		return
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !isImage(entry.Name()) {
			continue
		}
		path := filepath.Join(w.dir, entry.Name())
		present[path] = true
		if pending, ok := w.pending[path]; ok {
			w.finish(path, pending)
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		current := fileState{size: info.Size(), modTime: info.ModTime()}
		if previous, ok := w.states[path]; !ok || previous != current {
			w.states[path] = current
			continue
		}
		delete(w.states, path)
		w.ingest(path)
	}
	for path := range w.states {
		if !present[path] {
			delete(w.states, path)
		}
	}
}

func (w *watcher) ingest(path string) {
	raw, err := os.ReadFile(path)
	if err != nil {
		slog.Error("cannot read camera file", "path", path, "err", err)
		return
	}
	p, err := w.photos.Import(user.SU(), photo.Options{Source: photo.SourceCamera}, image.MemFile{
		Filename: filepath.Base(path), MimeTypeHint: mimeTypeOf(path), Bytes: raw,
	})
	if err != nil {
		slog.Error("cannot import camera file", "path", path, "err", err)
		return
	}

	opts := w.load()
	tpl := printing.TemplateByID(opts.AutoPrintTemplate).ID
	if opts.AutoPrintTemplate == "" {
		tpl = printing.TemplatePolaroid
	}
	pending := pendingPhoto{id: p.ID, template: tpl}
	w.pending[path] = pending
	slog.Info("imported camera photo", "path", path, "photo", string(p.ID))
	w.finish(path, pending)
}

func (w *watcher) finish(path string, pending pendingPhoto) {
	if !pending.printed && w.load().AutoPrint {
		if _, err := w.prints.Print(user.SU(), pending.id, pending.template); err != nil {
			// Der Auftrag kam nicht in die Warteschlange. Der nächste
			// Durchlauf versucht es erneut – etwa nachdem der Drucker wieder
			// erreichbar ist.
			slog.Error("cannot auto print camera photo", "photo", string(pending.id), "err", err)
			return
		}

		// Ab hier ist der Druck angestoßen und darf unter keinen Umständen
		// wiederholt werden.
		pending.printed = true
		w.pending[path] = pending
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Error("cannot remove imported camera file", "path", path, "err", err)
		return
	}
	delete(w.pending, path)
}

func isImage(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg", ".png":
		return true
	default:
		return false
	}
}

func mimeTypeOf(path string) string {
	if strings.EqualFold(filepath.Ext(path), ".png") {
		return "image/png"
	}
	return "image/jpeg"
}
