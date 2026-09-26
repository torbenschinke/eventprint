package device

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// RefillPaper meldet ein frisch eingelegtes Papierset.
type RefillPaper func(subject permission.Auditable) (Settings, error)

// NewRefillPaper erzeugt den [RefillPaper] Anwendungsfall.
func NewRefillPaper(mutex *sync.Mutex, store SettingsStore) RefillPaper {
	return func(subject permission.Auditable) (Settings, error) {
		if err := subject.Audit(PermRefillPaper); err != nil {
			return Settings{}, err
		}

		return modify(mutex, store, func(s *Settings) { s.PaperLeft = s.PaperCapacity })
	}
}
