package printing

import (
	"fmt"
	"sync"
	"time"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"
)

// NewRetry erzeugt den [Retry] Anwendungsfall. Typischer Anwendungsfall auf
// einer Feier: das Papier war leer, nach dem Nachlegen sollen die
// fehlgeschlagenen Aufträge ohne erneutes Suchen der Fotos nachlaufen.
func NewRetry(mutex *sync.Mutex, repo Repository, findJobByID FindJobByID, queue chan<- JobID) Retry {
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

		job.State = StateQueued
		job.Message = ""
		job.FinishedAt = time.Time{}

		mutex.Lock()
		if err := repo.Save(job); err != nil {
			mutex.Unlock()
			return fmt.Errorf("cannot save print job: %w", err)
		}
		mutex.Unlock()

		queue <- job.ID

		return nil
	}
}
