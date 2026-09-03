package printing

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/app/photo"
)

// Print stellt ein Foto mit dem gewählten Layout in die Druckwarteschlange
// und liefert die ID des angelegten Auftrags zurück. Der Aufruf kehrt sofort
// zurück, gedruckt wird asynchron.
type Print func(subject auth.Subject, id photo.ID, tpl TemplateID) (JobID, error)

// NewPrint erzeugt den [Print] Anwendungsfall.
func NewPrint(ctx context.Context, mutex *sync.Mutex, bus events.Bus, repo Repository, printer Printer, queue chan<- JobID) Print {
	return func(subject auth.Subject, id photo.ID, tpl TemplateID) (JobID, error) {
		if err := subject.Audit(PermPrint); err != nil {
			return "", err
		}

		now := time.Now()
		job := Job{
			ID:          NewJobID(now),
			Photo:       id,
			Template:    TemplateByID(tpl).ID,
			Printer:     printer.Name(),
			State:       StateQueued,
			RequestedBy: subject.Name(),
			CreatedAt:   now,
		}

		mutex.Lock()
		if err := repo.Save(job); err != nil {
			mutex.Unlock()
			return "", fmt.Errorf("cannot save print job: %w", err)
		}
		mutex.Unlock()

		if err := enqueue(ctx, queue, job.ID); err != nil {
			// Ein Auftrag, der nie in der Warteschlange ankam, darf nicht als
			// wartend zurückbleiben – sonst verspricht die Oberfläche einen
			// Ausdruck, der nie erfolgt.
			job.State = StateFailed
			job.Message = err.Error()
			job.FinishedAt = time.Now()

			mutex.Lock()
			if saveErr := repo.Save(job); saveErr != nil {
				slog.Error("cannot mark unqueued print job", "job", string(job.ID), "err", saveErr)
			}
			mutex.Unlock()

			return "", err
		}

		bus.Publish(JobQueued{Job: job.ID})

		return job.ID, nil
	}
}
