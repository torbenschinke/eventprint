package photo

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// MarkSeen nimmt Fotos aus dem Eingang.
//
// Ein Foto verlässt den Eingang, sobald jemand es für den Druck ausgewählt
// hat. Bis dahin zählt es als neu – auch nach Tagen, denn gedacht war es ja
// für den Druck.
type MarkSeen func(subject permission.Auditable, ids ...ID) error

// NewMarkSeen erzeugt den [MarkSeen] Anwendungsfall.
func NewMarkSeen(mutex *sync.Mutex, repo Repository) MarkSeen {
	return func(subject permission.Auditable, ids ...ID) error {
		if err := subject.Audit(PermMarkSeen); err != nil {
			return err
		}

		for _, id := range ids {
			if err := update(mutex, repo, id, func(p *Photo) { p.Unseen = false }); err != nil {
				return err
			}
		}

		return nil
	}
}
