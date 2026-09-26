package photo

import (
	"fmt"

	"go.wdy.de/nago/application/permission"
)

// FindByID liefert ein einzelnes Foto; false, wenn es nicht (mehr) existiert.
type FindByID func(subject permission.Auditable, id ID) (Photo, bool, error)

// NewFindByID erzeugt den [FindByID] Anwendungsfall.
func NewFindByID(repo Repository) FindByID {
	return func(subject permission.Auditable, id ID) (Photo, bool, error) {
		if err := subject.Audit(PermFindByID); err != nil {
			return Photo{}, false, err
		}

		opt, err := repo.FindByID(id)
		if err != nil {
			return Photo{}, false, fmt.Errorf("cannot load photo: %w", err)
		}

		if opt.IsNone() {
			return Photo{}, false, nil
		}

		// Private Fotos nur für die, die die Mediathek sehen dürfen; siehe
		// [Locate].
		p := opt.Unwrap()
		if !visibleTo(subject, p) {
			return Photo{}, false, nil
		}

		return p, true, nil
	}
}
