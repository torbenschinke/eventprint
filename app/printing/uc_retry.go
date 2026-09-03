package printing

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"
)

// Retry stellt einen fehlgeschlagenen Auftrag erneut in die Warteschlange.
type Retry func(subject auth.Subject, id JobID) error

// NewRetry erzeugt den [Retry] Anwendungsfall. Typischer Anwendungsfall auf
// einer Feier: das Papier war leer, nach dem Nachlegen sollen die
// fehlgeschlagenen Aufträge ohne erneutes Suchen der Fotos nachlaufen.
func NewRetry(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer, findJobByID FindJobByID, queue chan<- JobID) Retry {
	return func(subject auth.Subject, id JobID) error {
		if err := subject.Audit(PermRetry); err != nil {
			return err
		}

		optJob, err := findJobByID(subject, id)
		if err != nil {
			return err
		}

		if optJob.IsNone() {
			return std.NewLocalizedError("Unbekannter Auftrag", "Der Druckauftrag existiert nicht mehr.")
		}

		job := optJob.Unwrap()
		if !job.State.Done() {
			return std.NewLocalizedError("Auftrag läuft noch", "Dieser Druckauftrag wird gerade abgearbeitet.")
		}

		// Der vorherige Auftrag kann noch in der Warteschlange des
		// Druckdienstes liegen – etwa nach einem Timeout, bei dem die Fotobox
		// aufgegeben hat, CUPS aber nicht. Bliebe er stehen, ergäbe die
		// Wiederholung zwei Ausdrucke desselben Bildes.
		cancelPrinterJob(ctx, printer, job)

		previous := job

		job.PrinterJob = ""
		job.Reason = ""
		job.State = StateQueued
		job.Message = ""
		job.FinishedAt = time.Time{}

		mutex.Lock()
		if err := repo.Save(job); err != nil {
			mutex.Unlock()
			return fmt.Errorf("cannot save print job: %w", err)
		}
		mutex.Unlock()

		if err := enqueue(ctx, queue, job.ID); err != nil {
			// Der Auftrag darf nicht als wartend zurückbleiben, sonst zeigt
			// die Oberfläche dauerhaft "Wartet" für etwas, das niemand
			// abarbeitet.
			mutex.Lock()
			if saveErr := repo.Save(previous); saveErr != nil {
				slog.Error("cannot restore print job state", "job", string(job.ID), "err", saveErr)
			}
			mutex.Unlock()

			return err
		}

		return nil
	}
}
