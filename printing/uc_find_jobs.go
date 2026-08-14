package printing

import (
	"fmt"
	"iter"
	"slices"

	"github.com/worldiety/option"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/xiter"
)

// NewFindAllJobs erzeugt den [FindAllJobs] Anwendungsfall. Die Aufträge
// werden absteigend nach Erstellzeitpunkt geliefert.
func NewFindAllJobs(repo Repository) FindAllJobs {
	return func(subject auth.Subject) iter.Seq2[Job, error] {
		if err := subject.Audit(PermFindAllJobs); err != nil {
			return xiter.WithError[Job](err)
		}

		var jobs []Job
		for job, err := range repo.All() {
			if err != nil {
				return xiter.WithError[Job](fmt.Errorf("cannot read print jobs: %w", err))
			}

			jobs = append(jobs, job)
		}

		// repo.All liefert lexikographisch aufsteigend, die JobID ist
		// zeitlich sortierbar – umdrehen genügt.
		slices.Reverse(jobs)

		return func(yield func(Job, error) bool) {
			for _, job := range jobs {
				if !yield(job, nil) {
					return
				}
			}
		}
	}
}

// NewFindJobByID erzeugt den [FindJobByID] Anwendungsfall.
func NewFindJobByID(repo Repository) FindJobByID {
	return func(subject auth.Subject, id JobID) (option.Opt[Job], error) {
		if err := subject.Audit(PermFindJobByID); err != nil {
			return option.Opt[Job]{}, err
		}

		return repo.FindByID(id)
	}
}
