package photo

import (
	"fmt"
	"slices"

	"go.wdy.de/nago/application/permission"
)

// FindAll liefert die Fotos der Mediathek, die neuesten zuerst.
//
// Das ist die Sicht des Besitzers und schließt private Fotos ein. Im Kiosk
// hat niemand diese Berechtigung; Gäste bekommen [FindEvent].
type FindAll func(subject permission.Auditable, q Query) ([]Photo, error)

// NewFindAll erzeugt den [FindAll] Anwendungsfall.
func NewFindAll(repo Repository) FindAll {
	return func(subject permission.Auditable, q Query) ([]Photo, error) {
		if err := subject.Audit(PermFindAll); err != nil {
			return nil, err
		}

		return query(repo, q)
	}
}

// query liest das Repository und filtert. Die IDs sind zeitlich sortierbar,
// die umgekehrte Reihenfolge ist deshalb "neueste zuerst".
func query(repo Repository, q Query) ([]Photo, error) {
	var out []Photo
	for p, err := range repo.All() {
		if err != nil {
			return nil, fmt.Errorf("cannot read photos: %w", err)
		}

		if q.matches(p) {
			out = append(out, p)
		}
	}

	slices.SortFunc(out, func(a, b Photo) int {
		switch {
		case a.ID > b.ID:
			return -1
		case a.ID < b.ID:
			return 1
		default:
			return 0
		}
	})

	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}

	return out, nil
}
