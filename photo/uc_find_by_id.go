package photo

import (
	"github.com/worldiety/option"
	"go.wdy.de/nago/auth"
)

// NewFindByID erzeugt den [FindByID] Anwendungsfall.
func NewFindByID(repo Repository) FindByID {
	return func(subject auth.Subject, id ID) (option.Opt[Photo], error) {
		if err := subject.Audit(PermFindByID); err != nil {
			return option.Opt[Photo]{}, err
		}

		return repo.FindByID(id)
	}
}
