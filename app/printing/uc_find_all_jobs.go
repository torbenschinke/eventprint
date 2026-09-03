package printing

import (
	"cmp"
	"fmt"
	"iter"
	"slices"

	"go.wdy.de/nago/auth"
)

// FindAllJobs liefert alle Druckaufträge, beginnend mit dem neuesten.
type FindAllJobs func(subject auth.Subject) (iter.Seq2[Job, error], error)

// NewFindAllJobs erzeugt den [FindAllJobs] Anwendungsfall. Die Aufträge
// werden absteigend nach Erstellzeitpunkt geliefert.
func NewFindAllJobs(repo Repository) FindAllJobs {
	return func(subject auth.Subject) (iter.Seq2[Job, error], error) {
		// Die Verweigerung kommt sofort und nicht erst beim ersten Schritt
		// durch die Folge.
		if err := subject.Audit(PermFindAllJobs); err != nil {
			return nil, err
		}

		var jobs []Job
		for job, err := range repo.All() {
			if err != nil {
				return nil, fmt.Errorf("cannot read print jobs: %w", err)
			}

			jobs = append(jobs, job)
		}

		// Absteigend sortieren, nicht bloß umdrehen: In welcher Reihenfolge
		// eine Ablage ihre Einträge hergibt, ist ihre Sache und bei einem
		// Speicher über einer Map zufällig. Weil die [JobID] mit einem
		// nullgefüllten Zeitstempel beginnt, ist die lexikographische
		// Ordnung zugleich die zeitliche.
		slices.SortFunc(jobs, func(a, b Job) int { return cmp.Compare(b.ID, a.ID) })

		return func(yield func(Job, error) bool) {
			for _, job := range jobs {
				if !yield(job, nil) {
					return
				}
			}
		}, nil
	}
}
