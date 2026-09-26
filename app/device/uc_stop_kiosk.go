package device

import "go.wdy.de/nago/application/permission"

// StopKiosk beendet den Kiosk ohne Neustart.
//
// Das ist der Notausstieg der Betreuung. Der reguläre Weg bleibt das
// Ausschalten, und der braucht keine PIN.
type StopKiosk func(subject permission.Auditable) error

// NewStopKiosk erzeugt den [StopKiosk] Anwendungsfall.
func NewStopKiosk(kiosk KioskStore, lock *Lock) StopKiosk {
	return func(subject permission.Auditable) error {
		if err := subject.Audit(PermStopKiosk); err != nil {
			return err
		}

		if err := kiosk.Clear(); err != nil {
			return err
		}

		lock.Relock()

		return nil
	}
}
