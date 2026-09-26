package camera

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

const (
	// retryBase ist die Wartezeit vor dem ersten Wiederholungsversuch.
	retryBase = 500 * time.Millisecond

	// retryMax deckelt sie. Vorher versuchte der Verzeichnisdurchlauf eine
	// gescheiterte Übergabe jede Sekunde erneut, ohne Ende und ohne Abstand –
	// bei einem abgeschalteten Drucker ein Sturm aus Fehlversuchen.
	retryMax = 30 * time.Second

	// retryLimit begrenzt die Versuche je Bild. Danach bleibt die Datei
	// liegen, damit sie von Hand gerettet werden kann.
	retryLimit = 10
)

// job ist eine Aufnahme auf ihrem Weg von der Datei zum Gerät.
type job struct {
	path string

	// delivered hält fest, dass das Gerät die Aufnahme angenommen hat.
	//
	// Der Merker überlebt gescheiterte Löschversuche. Ohne ihn hätte eine
	// einzige klemmende Datei endlose Ausdrucke bedeutet, denn im Kiosk
	// druckt das Gerät jede angenommene Aufnahme.
	delivered bool

	attempts int
	nextTry  time.Time

	// vanished bedeutet, dass die Datei zwischen Meldung und Zugriff
	// verschwunden ist. Dann hat sie jemand anders erledigt.
	vanished bool
}

// worker übergibt Aufnahmen an das Gerät, ohne den Verzeichnisdurchlauf
// aufzuhalten.
type worker struct {
	load    LoadOptions
	deliver Deliver
	status  *status
	queue   <-chan string
	done    func(string)

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
		if j.vanished {
			// Nichts mehr zu tun, und nichts zu melden.
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
	if !j.delivered {
		if !w.deliverFile(j) {
			w.retry(j)
			return false
		}

		// Ab hier ist die Aufnahme angenommen und darf unter keinen
		// Umständen ein zweites Mal übergeben werden.
		j.delivered = true
		j.attempts = 0
		w.status.captured()
	}

	if err := os.Remove(j.path); err != nil && !os.IsNotExist(err) {
		slog.Error("cannot remove delivered camera file", "path", j.path, "err", err)
		w.retry(j)
		return false
	}

	return true
}

func (w *worker) deliverFile(j *job) bool {
	raw, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			// Die Datei ist weg, waehrend der Auftrag in der Warteschlange
			// stand. Das ist kein Fehler, sondern das Ende eines Wettlaufs:
			// Der Verzeichnisdurchlauf liest, der Worker loescht und gibt
			// frei, und der Durchlauf beansprucht danach den veralteten Pfad
			// noch einmal.
			slog.Debug("camera file vanished before delivery", "path", j.path)
			j.vanished = true
			return false
		}
		slog.Error("cannot read camera file", "path", j.path, "err", err)
		return false
	}

	// Zwischen Meldung und Lesen kann der Rest noch eingetroffen sein oder
	// die Übertragung abgebrochen sein. Ein zweiter Blick kostet nichts.
	if !complete(j.path) {
		slog.Warn("camera file is not complete yet", "path", j.path)
		return false
	}

	if err := w.deliver(filepath.Base(j.path), raw); err != nil {
		slog.Error("cannot deliver camera file", "path", j.path, "err", err)
		return false
	}

	slog.Info("delivered camera photo", "path", j.path)
	return true
}

func (w *worker) retry(j *job) {
	j.attempts++
	wait := retryBase << min(j.attempts-1, 16)
	if wait > retryMax {
		wait = retryMax
	}
	j.nextTry = time.Now().Add(wait)
}
