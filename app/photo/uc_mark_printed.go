package photo

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// MarkPrinted vermerkt ein gedrucktes Blatt an den Fotos darauf.
type MarkPrinted func(subject permission.Auditable, ids ...ID) error

// NewMarkPrinted erzeugt den [MarkPrinted] Anwendungsfall.
func NewMarkPrinted(mutex *sync.Mutex, repo Repository) MarkPrinted {
	return func(subject permission.Auditable, ids ...ID) error {
		if err := subject.Audit(PermMarkPrinted); err != nil {
			return err
		}

		for _, id := range ids {
			if err := update(mutex, repo, id, func(p *Photo) {
				p.Prints++
				p.Unseen = false
			}); err != nil {
				return err
			}
		}

		return nil
	}
}
