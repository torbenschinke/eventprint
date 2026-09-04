package camera

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

const (
	// retryBase ist die Wartezeit vor dem ersten Wiederholungsversuch.
	retryBase = 500 * time.Millisecond

	// retryMax deckelt sie. Vorher versuchte der Verzeichnisdurchlauf einen
	// gescheiterten Druck jede Sekunde erneut, ohne Ende und ohne Abstand –
	// bei einem abgeschalteten Drucker ein Sturm aus Fehlversuchen.
	retryMax = 30 * time.Second

	// retryLimit begrenzt die Versuche je Bild. Danach bleibt die Datei
	// liegen, damit sie von Hand gerettet werden kann.
	retryLimit = 10
)

// job ist eine Aufnahme auf ihrem Weg von der Datei in den Druckauftrag.
type job struct {
	path string

	// id ist leer, solange der Import noch aussteht.
	id       photo.ID
	template printing.TemplateID

	// printed hält fest, dass der Auftrag bereits eingereiht wurde.
	//
	// Der Merker überlebt gescheiterte Löschversuche. Ohne ihn hätte eine
	// einzige klemmende Datei endlosen Papierverbrauch bedeutet.
	printed bool

	attempts int
	nextTry  time.Time
}

// worker importiert und druckt, ohne den Verzeichnisdurchlauf aufzuhalten.
type worker struct {
	load   LoadOptions
	photos photo.UseCases
	prints printing.UseCases
	status *status
	queue  <-chan string
	done   func(string)

	pending []*job
}

func (w *worker) run(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		w.status.pending(len(w.pending))

		timer.Stop()
		select {
		case <-timer.C:
		default:
		}
		if wait, ok := w.untilNext(); ok {
			timer.Reset(wait)
		}

		select {
		case <-ctx.Done():
			return
		case path, ok := <-w.queue:
			if !ok {
				return
			}
			w.pending = append(w.pending, &job{path: path})
		case <-timer.C:
		}

		w.work(ctx)
	}
}

// untilNext liefert die Wartezeit bis zum nächsten fälligen Versuch.
func (w *worker) untilNext() (time.Duration, bool) {
	var next time.Time
	for _, j := range w.pending {
		if next.IsZero() || j.nextTry.Before(next) {
			next = j.nextTry
		}
	}
	if next.IsZero() {
		return 0, false
	}
	if wait := time.Until(next); wait > 0 {
		return wait, true
	}
	return time.Millisecond, true
}

// work arbeitet alle fälligen Aufträge ab.
func (w *worker) work(ctx context.Context) {
	keep := w.pending[:0]
	for _, j := range w.pending {
		if ctx.Err() != nil {
			return
		}
		if time.Now().Before(j.nextTry) {
			keep = append(keep, j)
			continue
		}
		if w.step(j) {
			w.done(j.path)
			continue
		}
		if j.attempts >= retryLimit {
			// Der Pfad wird ausdruecklich NICHT freigegeben. Sonst faende ihn
			// der naechste Verzeichnisdurchlauf wieder, reihte ihn erneut ein
			// und begaenne dieselben zehn Fehlversuche von vorn - endlos, denn
			// die Datei bleibt ja liegen. Aufgeben heisst hier: liegen lassen,
			// damit sie von Hand gerettet werden kann.
			slog.Error("giving up on camera capture", "path", j.path, "attempts", j.attempts)
			continue
		}
		keep = append(keep, j)
	}
	w.pending = keep
}

// step bringt einen Auftrag einen Schritt weiter und meldet, ob er fertig ist.
func (w *worker) step(j *job) bool {
	opts := w.load()

	if j.id == "" {
		id, ok := w.importFile(j)
		if !ok {
			w.retry(j)
			return false
		}
		j.id = id
		j.template = templateOf(opts)
		j.attempts = 0
		w.status.captured()
	}

	if !j.printed && opts.AutoPrint {
		if _, err := w.prints.Print(user.SU(), j.id, j.template); err != nil {
			slog.Error("cannot auto print camera photo", "photo", string(j.id), "err", err)
			w.retry(j)
			return false
		}

		// Ab hier ist der Druck angestoßen und darf unter keinen Umständen
		// wiederholt werden.
		j.printed = true
		j.attempts = 0
	}

	if err := os.Remove(j.path); err != nil && !os.IsNotExist(err) {
		slog.Error("cannot remove imported camera file", "path", j.path, "err", err)
		w.retry(j)
		return false
	}
	return true
}

func (w *worker) importFile(j *job) (photo.ID, bool) {
	raw, err := os.ReadFile(j.path)
	if err != nil {
		slog.Error("cannot read camera file", "path", j.path, "err", err)
		return "", false
	}

	// Zwischen Meldung und Lesen kann der Rest noch eingetroffen sein oder
	// die Übertragung abgebrochen sein. Ein zweiter Blick kostet nichts.
	if !complete(j.path) {
		slog.Warn("camera file is not complete yet", "path", j.path)
		return "", false
	}

	p, err := w.photos.Import(user.SU(), photo.Options{Source: photo.SourceCamera}, image.MemFile{
		Filename: filepath.Base(j.path), MimeTypeHint: mimeTypeOf(j.path), Bytes: raw,
	})
	if err != nil {
		slog.Error("cannot import camera file", "path", j.path, "err", err)
		return "", false
	}

	slog.Info("imported camera photo", "path", j.path, "photo", string(p.ID))
	return p.ID, true
}

func (w *worker) retry(j *job) {
	j.attempts++
	wait := retryBase << min(j.attempts-1, 16)
	if wait > retryMax {
		wait = retryMax
	}
	j.nextTry = time.Now().Add(wait)
}

func templateOf(opts Options) printing.TemplateID {
	if opts.AutoPrintTemplate == "" {
		return printing.TemplatePolaroid
	}
	return printing.TemplateByID(opts.AutoPrintTemplate).ID
}
