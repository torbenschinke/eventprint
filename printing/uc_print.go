package printing

import (
	"fmt"
	"sync"
	"time"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/events"

	"github.com/torbenschinke/eventprint/photo"
)

// NewPrint erzeugt den [Print] Anwendungsfall.
func NewPrint(mutex *sync.Mutex, bus events.Bus, repo Repository, printer Printer, queue chan<- JobID) Print {
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

		queue <- job.ID
		bus.Publish(JobQueued{Job: job.ID})

		return job.ID, nil
	}
}
