package device

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// SetPin legt die Betreuer-PIN fest.
//
// Nur im Heimbetrieb möglich, denn nur der Besitzer hat die Berechtigung. Im
// Kiosk bliebe sonst die Frage, wer eigentlich die neue PIN vergibt.
type SetPin func(subject permission.Auditable, pin string) error

// NewSetPin erzeugt den [SetPin] Anwendungsfall.
func NewSetPin(mutex *sync.Mutex, store SettingsStore) SetPin {
	return func(subject permission.Auditable, pin string) error {
		if err := subject.Audit(PermSetPin); err != nil {
			return err
		}

		hash, err := HashPin(pin)
		if err != nil {
			return err
		}

		_, err = modify(mutex, store, func(s *Settings) { s.Pin = hash })
		return err
	}
}
