package printing

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// worker arbeitet die Druckwarteschlange streng seriell ab. Das ist keine
// Einschränkung, sondern beabsichtigt: Es gibt genau einen Drucker, und ein
// Dye-Sublimation-Gerät kann ohnehin nur ein Bild gleichzeitig verarbeiten.
type worker struct {
	mutex         *sync.Mutex
	repo          Repository
	printer       Printer
	motifs        loadMotifs
	renderOptions func() RenderOptions
	observe       func(Job)
}

func newWorker(mutex *sync.Mutex, repo Repository, printer Printer, motifs loadMotifs, renderOptions func() RenderOptions, observe func(Job)) *worker {
	if observe == nil {
		observe = func(Job) {}
	}

	return &worker{
		mutex:         mutex,
		repo:          repo,
		printer:       printer,
		motifs:        motifs,
		renderOptions: renderOptions,
		observe:       observe,
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

	// Nur ein wartender Auftrag darf gedruckt werden. Landet dieselbe Kennung
	// ein zweites Mal in der Warteschlange – durch einen doppelten Klick, eine
	// Wiederholung oder die Wiederherstellung beim Start –, wäre das sonst ein
	// zweites Blatt Papier für dasselbe Bild.
	if job.State != StateQueued {
		slog.Warn("skipping print job that is not queued",
			"job", string(job.ID),
			"state", string(job.State),
		)

		return
	}

	job.State = StatePrinting
	job.Message = ""
	if !w.storeUnlessCanceled(ctx, job) {
		return
	}

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
	if !w.storeUnlessCanceled(ctx, job) {
		return
	}

	outcome := w.await(ctx, job)

	// Während des Wartens kann der Auftrag abgebrochen worden sein. Dann
	// steht dieser Ausgang bereits fest und darf nicht überschrieben werden.
	if current, ok := w.load(job.ID); ok && current.Reason == reasonCanceled {
		return
	}

	job.FinishedAt = time.Now()
	job.Reason = outcome.Reason

	if outcome.Success {
		job.State = StateDone
		// Die Annahmebestätigung von lp ist jetzt bedeutungslos – der
		// Zustand "Fertig" sagt alles. Nur eine echte Meldung des Druckers
		// bleibt stehen.
		job.Message = outcome.Message

		slog.Info("print job done", "job", string(job.ID), "printerJob", job.PrinterJob, "format", string(job.Layout.Format), "design", string(job.Layout.Design))
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
	w.observe(job)
}

// render lädt die Motive, legt das Blatt an und übergibt es dem Drucker.
func (w *worker) render(ctx context.Context, job Job) (Result, error) {
	sheet, err := w.motifs(job.Photos, job.Layout)
	if err != nil {
		return Result{}, err
	}

	buf, err := RenderSheet(sheet, NativeRaster4x6, w.renderOptions())
	if err != nil {
		return Result{}, err
	}

	name := string(job.ID)
	if fp, ok := w.printer.(FinishPrinter); ok {
		return fp.PrintFinish(ctx, buf, name, job.Layout.Finish)
	}

	return w.printer.Print(ctx, buf, name)
}

// storeUnlessCanceled speichert den Zwischenstand, sofern der Auftrag nicht
// inzwischen abgebrochen wurde, und meldet, ob es weitergehen darf.
//
// Der Worker arbeitet mit einer Kopie, die er vor dem Rendern geladen hat.
// Ein Abbruch in dieser Zeit stünde nur im Repository; ein blindes Speichern
// der Kopie löschte ihn und das Blatt käme trotzdem. Wurde der Auftrag schon
// an CUPS übergeben, wird er dort zurückgenommen.
func (w *worker) storeUnlessCanceled(ctx context.Context, job Job) bool {
	w.mutex.Lock()
	opt, err := w.repo.FindByID(job.ID)
	if err == nil && opt.IsSome() && opt.Unwrap().Reason == reasonCanceled {
		w.mutex.Unlock()
		cancelPrinterJob(ctx, w.printer, job)
		slog.Info("print job canceled while in progress", "job", string(job.ID))

		return false
	}

	if err := w.repo.Save(job); err != nil {
		slog.Error("cannot save print job", "job", string(job.ID), "err", err)
	}
	w.mutex.Unlock()

	return true
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

// maxRecoverAge begrenzt, wie alt ein wartender Auftrag sein darf, um beim
// Start noch gedruckt zu werden.
//
// Ein Auftrag, der beim letzten Herunterfahren offen war, ist nach einer
// Nachtruhe kein Auftrag mehr, sondern eine Überraschung: Niemand steht dann
// noch am Drucker, und das Bild gehört zu einem Gast, der längst gegangen
// ist. Solche Aufträge werden ausgewiesen, nicht ausgeführt.
const maxRecoverAge = 15 * time.Minute

// recoverStaleJobs behandelt Aufträge, die beim letzten Herunterfahren noch
// offen waren. Junge wartende Aufträge werden erneut eingereiht; ein Auftrag,
// der mitten im Druck stand, wird als fehlgeschlagen markiert, weil unbekannt
// ist, ob das Papier bereits verbraucht wurde.
//
// In beiden Fällen wird ein eventuell noch vorhandener Auftrag des
// Druckdienstes storniert. Ohne das würde er beim nächsten Erreichen des
// Druckers zusätzlich zum neuen Versuch ausgegeben.
func recoverStaleJobs(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer, queue chan<- JobID) {
	mutex.Lock()
	defer mutex.Unlock()

	var requeue []JobID

	cutoff := time.Now().Add(-maxRecoverAge)

	for job, err := range repo.All() {
		if err != nil {
			slog.Error("cannot scan print jobs on startup", "err", err)
			return
		}

		if job.State.Done() {
			continue
		}

		// Der frühere Auftrag im Druckdienst ist in jedem Fall hinfällig: Er
		// gehört zu einem Lauf, dessen Ausgang niemand mehr kennt.
		cancelPrinterJob(ctx, printer, job)

		switch {
		case job.State == StatePrinting:
			job.State = StateFailed
			job.Message = "Der Auftrag wurde durch einen Neustart der Fotobox unterbrochen."

		case job.CreatedAt.Before(cutoff):
			job.State = StateFailed
			job.Message = "Der Auftrag lag beim Neustart zu lange zurück und wurde nicht mehr automatisch gedruckt. Über 'Wiederholen' lässt er sich bewusst erneut ausgeben."

		default:
			job.PrinterJob = ""
			if err := repo.Save(job); err != nil {
				slog.Error("cannot reset recovered print job", "job", string(job.ID), "err", err)
				continue
			}

			requeue = append(requeue, job.ID)

			continue
		}

		job.PrinterJob = ""
		job.FinishedAt = time.Now()

		if err := repo.Save(job); err != nil {
			slog.Error("cannot mark stale print job", "job", string(job.ID), "err", err)
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

// cancelPrinterJob nimmt den zugehörigen Auftrag aus der Warteschlange des
// Druckdienstes, sofern der Drucker das unterstützt.
func cancelPrinterJob(ctx context.Context, printer Printer, job Job) {
	if job.PrinterJob == "" {
		return
	}

	canceller, ok := printer.(Canceller)
	if !ok {
		return
	}

	if err := canceller.Cancel(ctx, job.PrinterJob); err != nil {
		slog.Warn("cannot cancel leftover printer job",
			"job", string(job.ID),
			"printerJob", job.PrinterJob,
			"err", err,
		)
	}
}
