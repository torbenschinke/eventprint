// Package camera detects USB/PTP cameras through gphoto2 and imports tethered captures.
package camera

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

type Options struct {
	AutoPrint         bool
	AutoPrintTemplate printing.TemplateID

	// ScanInterval ist der Abstand des Rückfall-Durchlaufs über das
	// Verzeichnis. Der Normalfall kommt über fsnotify, dieser Takt fängt nur
	// ab, was der Kernel nicht meldet.
	ScanInterval time.Duration

	// DetectInterval ist der Abstand der Kamerasuche, solange keine Kamera am
	// USB hängt. Er gilt ausdrücklich NICHT für den Wiederaufbau eines
	// abgerissenen Tetherings.
	DetectInterval time.Duration
}

type LoadOptions func() Options

// Monitor ist die Kameraanbindung: Erkennung, Tethering, Übernahme, Druck.
type Monitor struct {
	dir     string
	load    LoadOptions
	photos  photo.UseCases
	prints  printing.UseCases
	status  *status
	runner  commandRunner
	queue   chan string
	watcher *watcher
}

// New baut die Kameraanbindung auf, ohne sie zu starten.
func New(dir string, load LoadOptions, photos photo.UseCases, prints printing.UseCases) *Monitor {
	st := &status{value: Status{State: StateSearching}}
	m := &Monitor{
		dir:    dir,
		load:   load,
		photos: photos,
		prints: prints,
		status: st,
		runner: execRunner{},
		queue:  make(chan string, queueDepth),
	}
	m.watcher = &watcher{dir: dir, states: map[string]fileState{}, taken: map[string]bool{}, queue: m.queue}
	return m
}

// Status meldet den Zustand der Kamera an die Oberfläche. Er darf aus jedem
// Thread gelesen werden.
func (m *Monitor) Status() Status {
	return m.status.get()
}

// Run hält Erkennung, Tethering, Übernahme und Druck am Leben, bis ctx endet.
func (m *Monitor) Run(ctx context.Context) {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		slog.Error("cannot create camera directory", "dir", m.dir, "err", err)
		m.status.failed("", "", "Der Ordner für die Kamerabilder lässt sich nicht anlegen.")
		return
	}

	sup := &supervisor{dir: m.dir, load: m.load, runner: m.runner, status: m.status}
	go sup.run(ctx)

	// Import und Druck laufen in einem eigenen Thread. Vorher geschah beides
	// mitten im Verzeichnisdurchlauf: Solange ein Bild geschrieben oder ein
	// Druckauftrag eingereiht wurde, sah niemand nach neuen Aufnahmen. Ein
	// hängender Drucker legte damit die gesamte Bildannahme still.
	w := &worker{
		load: m.load, photos: m.photos, prints: m.prints,
		status: m.status, queue: m.queue, done: m.watcher.release,
	}
	go w.run(ctx)

	m.watcher.run(ctx, m.load)
}

// Run ist der frühere Einstieg und bleibt als Abkürzung erhalten.
func Run(ctx context.Context, dir string, load LoadOptions, photos photo.UseCases, prints printing.UseCases) {
	New(dir, load, photos, prints).Run(ctx)
}

// queueDepth puffert Aufnahmen, die schneller eintreffen, als sie gedruckt
// werden. Eine Serienaufnahme darf nicht dazu führen, dass der Durchlauf
// blockiert.
const queueDepth = 64

type fileState struct {
	size    int64
	modTime time.Time
}

// watcher meldet fertige Dateien an den Worker.
type watcher struct {
	dir   string
	queue chan<- string

	states map[string]fileState

	// taken hält fest, welche Dateien bereits übergeben sind. Ohne diesen
	// Merker würde derselbe Pfad bei jedem Durchlauf erneut in die
	// Warteschlange wandern, solange der Worker noch daran arbeitet.
	//
	// Der Worker gibt Einträge aus seinem eigenen Thread frei, deshalb der
	// Mutex.
	mu    sync.Mutex
	taken map[string]bool
}

func (w *watcher) claim(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.taken[path] {
		return false
	}
	w.taken[path] = true
	return true
}

// release gibt einen Pfad wieder frei, nachdem der Worker fertig ist.
func (w *watcher) release(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.taken, path)
}

func (w *watcher) forgetMissing(present map[string]bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path := range w.taken {
		if !present[path] {
			delete(w.taken, path)
		}
	}
}

// run wartet auf Meldungen des Kernels und läuft zusätzlich im Takt.
func (w *watcher) run(ctx context.Context, load LoadOptions) {
	w.scan()

	events := w.notify(ctx)
	timer := time.NewTimer(scanInterval(load))
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			w.scan()
		case <-timer.C:
			w.scan()
		}
		// Der Takt wird bei jedem Durchlauf neu gelesen. Vorher stand er ein
		// einziges Mal beim Start fest, eine geänderte Einstellung wirkte nie.
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(scanInterval(load))
	}
}

func scanInterval(load LoadOptions) time.Duration {
	if interval := load().ScanInterval; interval > 0 {
		return interval
	}
	return time.Second
}

// notify liefert Schreibmeldungen des Kernels für das Verzeichnis.
//
// Fällt das aus, bleibt der getaktete Durchlauf als Rückfall. Deshalb ist ein
// Fehler hier kein Grund aufzuhören.
func (w *watcher) notify(ctx context.Context) <-chan struct{} {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Warn("cannot watch camera directory, falling back to polling", "err", err)
		return nil
	}
	if err := fsw.Add(w.dir); err != nil {
		slog.Warn("cannot watch camera directory, falling back to polling", "dir", w.dir, "err", err)
		_ = fsw.Close()
		return nil
	}

	out := make(chan struct{}, 1)
	go func() {
		defer close(out)
		defer func() { _ = fsw.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-fsw.Events:
				if !ok {
					return
				}
				select {
				case out <- struct{}{}:
				default:
				}
			case err, ok := <-fsw.Errors:
				if !ok {
					return
				}
				slog.Warn("camera directory watch failed", "err", err)
			}
		}
	}()
	return out
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

		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}

		// Vollständigkeit wird am Bild selbst geprüft, nicht an zwei gleichen
		// Verzeichniseinträgen. Die alte Regel kostete immer einen zusätzlichen
		// Takt und war trotzdem unsicher: Stockte der USB-Transfer eine Sekunde
		// lang, sahen zwei Durchläufe dieselbe Größe und ein halbes JPEG ging
		// in den Import.
		if !complete(path) {
			w.states[path] = fileState{size: info.Size(), modTime: info.ModTime()}
			continue
		}

		delete(w.states, path)
		if !w.claim(path) {
			continue
		}
		select {
		case w.queue <- path:
		default:
			// Die Warteschlange ist voll. Der Pfad bleibt liegen und kommt im
			// nächsten Durchlauf erneut dran.
			w.release(path)
		}
	}

	for path := range w.states {
		if !present[path] {
			delete(w.states, path)
		}
	}
	w.forgetMissing(present)
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
