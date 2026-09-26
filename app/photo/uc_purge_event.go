package photo

import (
	"errors"
	"fmt"
	"sync"

	"go.wdy.de/nago/application/permission"
)

// PurgeEvent entfernt alle Fotos einer Feier samt Originalen und liefert
// deren Zahl.
//
// Gedacht für den Schluss: erst auf den USB-Stick, dann Platz für die nächste
// Feier. Private Fotos bleiben unberührt.
type PurgeEvent func(subject permission.Auditable, event EventID) (int, error)

// NewPurgeEvent erzeugt den [PurgeEvent] Anwendungsfall.
func NewPurgeEvent(mutex *sync.Mutex, repo Repository, originals Originals) PurgeEvent {
	return func(subject permission.Auditable, event EventID) (int, error) {
		if err := subject.Audit(PermPurgeEvent); err != nil {
			return 0, err
		}

		if event == "" {
			return 0, errors.New("ohne Feier gibt es nichts abzuschließen")
		}

		victims, err := query(repo, Query{Scope: ScopeEvent, Event: event})
		if err != nil {
			return 0, err
		}

		mutex.Lock()
		defer mutex.Unlock()

		for _, p := range victims {
			if err := originals.Remove(p.File); err != nil {
				return 0, fmt.Errorf("cannot remove original: %w", err)
			}

			if err := repo.DeleteByID(p.ID); err != nil {
				return 0, fmt.Errorf("cannot delete photo: %w", err)
			}
		}

		return len(victims), nil
	}
}
