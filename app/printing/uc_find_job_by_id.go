package printing

import (
	"github.com/worldiety/option"
	"go.wdy.de/nago/application/permission"
)

// FindJobByID liefert einen einzelnen Druckauftrag.
type FindJobByID func(subject permission.Auditable, id JobID) (option.Opt[Job], error)

// NewFindJobByID erzeugt den [FindJobByID] Anwendungsfall.
func NewFindJobByID(repo Repository) FindJobByID {
	return func(subject permission.Auditable, id JobID) (option.Opt[Job], error) {
		if err := subject.Audit(PermFindJobByID); err != nil {
			return option.Opt[Job]{}, err
		}

		return repo.FindByID(id)
	}
}
