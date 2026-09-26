package photo

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// SetFavorite markiert Fotos als Favorit oder nimmt die Markierung zurück.
type SetFavorite func(subject permission.Auditable, favorite bool, ids ...ID) error

// NewSetFavorite erzeugt den [SetFavorite] Anwendungsfall.
func NewSetFavorite(mutex *sync.Mutex, repo Repository) SetFavorite {
	return func(subject permission.Auditable, favorite bool, ids ...ID) error {
		if err := subject.Audit(PermSetFavorite); err != nil {
			return err
		}

		for _, id := range ids {
			if err := update(mutex, repo, id, func(p *Photo) { p.Favorite = favorite }); err != nil {
				return err
			}
		}

		return nil
	}
}
