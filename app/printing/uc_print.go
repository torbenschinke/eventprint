package printing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
)

// PrintCmd beschreibt einen Druck aus dem Druck-Studio.
type PrintCmd struct {
	// Photos sind die gewählten Fotos. Das Layout entscheidet, wie viele
	// davon auf ein Blatt passen.
	Photos []photo.ID

	Layout Layout

	// Copies ist die Anzahl je Blatt.
	Copies int
}

// Batch ist die Quittung eines Druckvorgangs.
type Batch struct {
	ID   BatchID
	Jobs []JobID
}

// MaxCopies begrenzt einen Druckvorgang. Ein Tippfehler im Zähler soll kein
// ganzes Papierset kosten.
const MaxCopies = 20

// Print stellt die Blätter eines Druckvorgangs in die Warteschlange.
//
// Der Aufruf kehrt sofort zurück; gedruckt wird im Hintergrund. Jedes Blatt
// ist ein eigener Auftrag, damit ein Papierwechsel mitten im Stapel genau das
// fehlende Blatt wiederholt und keines doppelt entsteht.
type Print func(subject permission.Auditable, cmd PrintCmd) (Batch, error)

// NewPrint erzeugt den [Print] Anwendungsfall.
func NewPrint(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer, queue chan<- JobID) Print {
	return func(subject permission.Auditable, cmd PrintCmd) (Batch, error) {
		if err := subject.Audit(PermPrint); err != nil {
			return Batch{}, err
		}

		return enqueueBatch(ctx, mutex, repo, printer, queue, cmd)
	}
}

// enqueueBatch zerlegt einen Druckvorgang in Blätter und reiht sie ein.
func enqueueBatch(ctx context.Context, mutex *sync.Mutex, repo Repository, printer Printer, queue chan<- JobID, cmd PrintCmd) (Batch, error) {
	if len(cmd.Photos) == 0 {
		return Batch{}, errors.New("es ist kein Foto ausgewählt")
	}

	layout := cmd.Layout.Normalized()
	copies := min(max(cmd.Copies, 1), MaxCopies)

	// Ein Blatt nimmt so viele verschiedene Motive auf, wie das Format
	// vorsieht. Bleiben Felder frei, füllt der Renderer sie mit den
	// vorhandenen Motiven auf, statt weiße Löcher zu drucken.
	var sheets [][]photo.ID
	for i := 0; i < len(cmd.Photos); i += layout.Motifs() {
		sheets = append(sheets, cmd.Photos[i:min(i+layout.Motifs(), len(cmd.Photos))])
	}

	now := time.Now()
	batch := Batch{ID: BatchID(NewJobID(now))}
	total := len(sheets) * copies

	var jobs []Job
	n := 0
	for _, ids := range sheets {
		for range copies {
			n++
			// Die Kennungen sind zeitlich sortiert. Ein Millisekundenschritt je
			// Blatt hält die Blätter eines Vorgangs in ihrer Reihenfolge.
			jobs = append(jobs, Job{
				ID:        NewJobID(now.Add(time.Duration(n) * time.Millisecond)),
				Photos:    ids,
				Layout:    layout,
				Batch:     batch.ID,
				Sheet:     n,
				Sheets:    total,
				Printer:   printer.Name(),
				State:     StateQueued,
				CreatedAt: now,
			})
		}
	}

	mutex.Lock()
	for _, job := range jobs {
		if err := repo.Save(job); err != nil {
			mutex.Unlock()
			return Batch{}, fmt.Errorf("cannot save print job: %w", err)
		}
	}
	mutex.Unlock()

	for i, job := range jobs {
		if err := enqueue(ctx, queue, job.ID); err != nil {
			// Was nicht mehr in die Schlange passt, wird als gescheitert
			// vermerkt statt stillschweigend liegen zu bleiben. Über
			// "Wiederholen" lässt es sich später bewusst nachholen.
			mutex.Lock()
			for _, rest := range jobs[i:] {
				rest.State = StateFailed
				rest.Message = err.Error()
				rest.FinishedAt = time.Now()
				if saveErr := repo.Save(rest); saveErr != nil {
					slog.Error("cannot mark unqueued print job", "job", string(rest.ID), "err", saveErr)
				}
			}
			mutex.Unlock()

			return batch, err
		}

		batch.Jobs = append(batch.Jobs, job.ID)
	}

	return batch, nil
}
