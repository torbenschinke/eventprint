package printing

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/photo"
)

// worker arbeitet die Druckwarteschlange streng seriell ab. Das ist keine
// Einschränkung, sondern beabsichtigt: Es gibt genau einen Drucker, und ein
// Dye-Sublimation-Gerät kann ohnehin nur ein Bild gleichzeitig verarbeiten.
type worker struct {
	mutex        *sync.Mutex
	bus          events.Bus
	repo         Repository
	printer      Printer
	openOriginal photo.OpenOriginal

	// raster wird je Auftrag ausgewertet, damit eine geänderte Einstellung
	// ohne Neustart wirkt.
	raster func() Raster
}

func newWorker(mutex *sync.Mutex, bus events.Bus, repo Repository, printer Printer, openOriginal photo.OpenOriginal, raster func() Raster) *worker {
	return &worker{
		mutex:        mutex,
		bus:          bus,
		repo:         repo,
		printer:      printer,
		openOriginal: openOriginal,
		raster:       raster,
	}
}

func (w *worker) run(ctx context.Context, queue <-chan JobID) {
	slog.Info("print worker started", "printer", w.printer.Name())

	for {
		select {
		case <-ctx.Done():
			slog.Info("print worker stopped")
			return

		case id := <-queue:
			w.process(ctx, id)
		}
	}
}

// process führt genau einen Auftrag aus. Fehler beenden den Worker nie, sie
// landen am Auftrag und damit auf der Druckstatus-Seite.
func (w *worker) process(ctx context.Context, id JobID) {
	job, ok := w.load(id)
	if !ok {
		return
	}

	job.State = StatePrinting
	job.Message = ""
	w.store(job)

	res, err := w.render(ctx, job)
	if err != nil {
		job.State = StateFailed
		job.Message = err.Error()
		job.FinishedAt = time.Now()

		slog.Error("print job failed", "job", string(job.ID), "err", err)
		w.finish(job)

		return
	}

	// Die Übergabe hat geklappt. Ab hier ist der Auftrag in der Hand von
	// CUPS – sichtbar für die Bedienung, die ihn im Zweifel dort wiederfindet.
	job.PrinterJob = res.JobID
	job.Message = res.Message
	w.store(job)

	outcome := w.await(ctx, job)
	job.FinishedAt = time.Now()
	job.Reason = outcome.Reason

	if outcome.Success {
		job.State = StateDone
		// Die Annahmebestätigung von lp ist jetzt bedeutungslos – der
		// Zustand "Fertig" sagt alles. Nur eine echte Meldung des Druckers
		// bleibt stehen.
		job.Message = outcome.Message

		slog.Info("print job done", "job", string(job.ID), "printerJob", job.PrinterJob, "template", string(job.Template))
	} else {
		job.State = StateFailed
		job.Message = outcome.Message
		if job.Message == "" {
			job.Message = "Der Drucker hat den Auftrag nicht ausgeführt (" + outcome.Reason + ")."
		}

		slog.Error("print job rejected by printer",
			"job", string(job.ID),
			"printerJob", job.PrinterJob,
			"reason", outcome.Reason,
			"message", outcome.Message,
		)
	}

	w.finish(job)
}

// await fragt den tatsächlichen Ausgang beim Drucker ab.
//
// Kann der Drucker keine Auskunft geben – etwa der Testbetrieb –, gilt die
// Übergabe als Erfolg. Für alle echten Kanäle ist die Auskunft
// verpflichtend, denn die Annahme durch lp sagt nichts über den Ausdruck aus.
func (w *worker) await(ctx context.Context, job Job) Outcome {
	tracker, ok := w.printer.(Tracker)
	if !ok {
		return Outcome{Done: true, Success: true}
	}

	return tracker.Await(ctx, job.PrinterJob)
}

func (w *worker) finish(job Job) {
	w.store(job)
	w.bus.Publish(JobFinished{Job: job.ID, State: job.State, Message: job.Message})
}

// render lädt das Original, wendet das Layout an und übergibt das Ergebnis an
// den Drucker.
func (w *worker) render(ctx context.Context, job Job) (Result, error) {
	// Der Worker läuft ohne Nutzerkontext, deshalb als Systemnutzer.
	subject := user.SU()

	optReader, err := w.openOriginal(subject, job.Photo)
	if err != nil {
		return Result{}, err
	}

	if optReader.IsNone() {
		return Result{}, errPhotoGone
	}

	reader := optReader.Unwrap()
	defer func() {
		_ = reader.Close()
	}()

	buf, err := Render(reader, job.Template, w.raster())
	if err != nil {
		return Result{}, err
	}

	return w.printer.Print(ctx, buf, string(job.Photo))
}

func (w *worker) load(id JobID) (Job, bool) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	optJob, err := w.repo.FindByID(id)
	if err != nil {
		slog.Error("cannot load print job", "job", string(id), "err", err)
		return Job{}, false
	}

	if optJob.IsNone() {
		slog.Warn("print job vanished", "job", string(id))
		return Job{}, false
	}

	return optJob.Unwrap(), true
}

func (w *worker) store(job Job) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if err := w.repo.Save(job); err != nil {
		slog.Error("cannot save print job", "job", string(job.ID), "err", err)
	}
}

// recoverStaleJobs behandelt Aufträge, die beim letzten Herunterfahren noch
// offen waren. Wartende Aufträge werden erneut eingereiht; ein Auftrag, der
// mitten im Druck stand, wird als fehlgeschlagen markiert, weil unbekannt
// ist, ob das Papier bereits verbraucht wurde.
func recoverStaleJobs(mutex *sync.Mutex, repo Repository, queue chan<- JobID) {
	mutex.Lock()
	defer mutex.Unlock()

	var requeue []JobID

	for job, err := range repo.All() {
		if err != nil {
			slog.Error("cannot scan print jobs on startup", "err", err)
			return
		}

		switch job.State {
		case StateQueued:
			requeue = append(requeue, job.ID)

		case StatePrinting:
			job.State = StateFailed
			job.Message = "Der Auftrag wurde durch einen Neustart der Fotobox unterbrochen."
			job.FinishedAt = time.Now()
			if err := repo.Save(job); err != nil {
				slog.Error("cannot mark stale print job", "job", string(job.ID), "err", err)
			}
		}
	}

	for _, id := range requeue {
		select {
		case queue <- id:
		default:
			// Warteschlange voll – der Rest bleibt auf StateQueued stehen und
			// kann über die Oberfläche erneut gestartet werden.
			slog.Warn("print queue full while recovering", "job", string(id))
			return
		}
	}
}
