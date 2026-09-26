package device

import "go.wdy.de/nago/application/permission"

// CurrentKiosk liefert die laufende Feier; ein leerer Wert bedeutet
// Heimbetrieb.
type CurrentKiosk func(subject permission.Auditable) (Kiosk, error)

// NewCurrentKiosk erzeugt den [CurrentKiosk] Anwendungsfall.
func NewCurrentKiosk(kiosk KioskStore) CurrentKiosk {
	return func(subject permission.Auditable) (Kiosk, error) {
		if err := subject.Audit(PermCurrentKiosk); err != nil {
			return Kiosk{}, err
		}

		return kiosk.Load()
	}
}
