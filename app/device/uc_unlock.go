package device

import "go.wdy.de/nago/application/permission"

// Unlock schaltet mit der PIN die Betreuung frei.
//
// Gäste haben diese Berechtigung, denn vor dem Kiosk steht niemand anderes.
// Die Sperre nach Fehlversuchen gilt für das ganze Gerät: Es gibt nur einen
// Bildschirm, und wer rät, rät an ihm.
type Unlock func(subject permission.Auditable, pin string) error

// NewUnlock erzeugt den [Unlock] Anwendungsfall.
func NewUnlock(store SettingsStore, lock *Lock) Unlock {
	return func(subject permission.Auditable, pin string) error {
		if err := subject.Audit(PermUnlock); err != nil {
			return err
		}

		s, err := store.Load()
		if err != nil {
			return err
		}

		return lock.verify(pin, s.Pin)
	}
}
