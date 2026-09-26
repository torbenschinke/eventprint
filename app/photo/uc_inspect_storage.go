package photo

import (
	"go.wdy.de/nago/application/permission"
)

// InspectStorage beschreibt die Belegung der Speicherkarte.
//
// Ohne diese Auskunft ist der Zeitpunkt zum Aufräumen nicht bestimmbar – bis
// mitten auf einer Feier kein Foto mehr angenommen wird.
type InspectStorage func(subject permission.Auditable) (Usage, error)

// NewInspectStorage erzeugt den [InspectStorage] Anwendungsfall.
func NewInspectStorage(originals Originals) InspectStorage {
	return func(subject permission.Auditable) (Usage, error) {
		if err := subject.Audit(PermInspectStorage); err != nil {
			return Usage{}, err
		}

		return originals.Usage()
	}
}
