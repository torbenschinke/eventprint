package photo

import (
	"fmt"
	"sync"

	"go.wdy.de/nago/application/permission"
)

// Delete entfernt Fotos samt Original.
//
// Anders als früher bleibt kein Archivexemplar zurück: Auf einem Heimdrucker
// ist Löschen gemeint, wie es dasteht, und die Speicherkarte ist endlich.
type Delete func(subject permission.Auditable, ids ...ID) error

// NewDelete erzeugt den [Delete] Anwendungsfall.
func NewDelete(mutex *sync.Mutex, repo Repository, originals Originals) Delete {
	return func(subject permission.Auditable, ids ...ID) error {
		if err := subject.Audit(PermDelete); err != nil {
			return err
		}

		mutex.Lock()
		defer mutex.Unlock()

		for _, id := range ids {
			opt, err := repo.FindByID(id)
			if err != nil {
				return fmt.Errorf("cannot load photo: %w", err)
			}

			if opt.IsNone() {
				continue
			}

			if err := originals.Remove(opt.Unwrap().File); err != nil {
				return fmt.Errorf("cannot remove original: %w", err)
			}

			if err := repo.DeleteByID(id); err != nil {
				return fmt.Errorf("cannot delete photo: %w", err)
			}
		}

		return nil
	}
}
