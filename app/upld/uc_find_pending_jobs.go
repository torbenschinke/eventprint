package upld

import "go.wdy.de/nago/auth"

// FindPendingJobs liefert die Aufträge, die für die anfragende Fotobox
// bereitliegen.
type FindPendingJobs func(subject auth.Subject) ([]Job, error)

// NewFindPendingJobs erzeugt den [FindPendingJobs] Anwendungsfall.
func NewFindPendingJobs(registry *Registry) FindPendingJobs {
	return func(subject auth.Subject) ([]Job, error) {
		if err := subject.Audit(PermPollJobs); err != nil {
			return nil, err
		}

		return registry.Pending(tokenOf(subject))
	}
}
