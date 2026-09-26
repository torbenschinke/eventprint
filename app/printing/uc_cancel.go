package printing

import (
	"context"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"
)

// Cancel bricht Aufträge ab, die noch nicht gedruckt sind.
//
// Ein wartender Auftrag verlässt die Schlange; einer, der gerade beim Drucker
// liegt, wird dort zurückgenommen. Ein Blatt, das der Drucker schon zieht,
// lässt sich nicht aufhalten – der Auftrag wird trotzdem als abgebrochen
// vermerkt, damit er nicht später von selbst wiederkommt.
type Cancel func(subject permission.Auditable, ids ...JobID) error

// reasonCanceled kennzeichnet einen von Hand abgebrochenen Auftrag.
const reasonCanceled = "canceled-by-user"

// NewCancel erzeugt den [Cancel] Anwendungsfall.
func NewCancel(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer) Cancel {
	return func(subject permission.Auditable, ids ...JobID) error {
		if err := subject.Audit(PermCancel); err != nil {
			return err
		}

		for _, id := range ids {
			mutex.Lock()
			opt, err := repo.FindByID(id)
			if err != nil || opt.IsNone() {
				mutex.Unlock()
				if err != nil {
					return err
				}

				continue
			}

			job := opt.Unwrap()
			if job.State.Done() {
				mutex.Unlock()
				continue
			}

			job.State = StateFailed
			job.Message = "Abgebrochen."
			job.Reason = reasonCanceled
			job.FinishedAt = time.Now()
			err = repo.Save(job)
			mutex.Unlock()

			if err != nil {
				return err
			}

			cancelPrinterJob(ctx, printer, job)
		}

		return nil
	}
}
