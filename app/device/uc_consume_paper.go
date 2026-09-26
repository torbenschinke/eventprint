package device

import (
	"sync"

	"go.wdy.de/nago/application/permission"
)

// ConsumePaper zieht gedruckte Blätter vom Vorrat ab.
//
// Der Zähler geht nicht unter null: Wer das Nachlegen nicht meldet, druckt
// trotzdem weiter, und eine negative Zahl sähe nach einem Defekt aus.
type ConsumePaper func(subject permission.Auditable, sheets int) error

// NewConsumePaper erzeugt den [ConsumePaper] Anwendungsfall.
func NewConsumePaper(mutex *sync.Mutex, store SettingsStore) ConsumePaper {
	return func(subject permission.Auditable, sheets int) error {
		if err := subject.Audit(PermConsumePaper); err != nil {
			return err
		}

		_, err := modify(mutex, store, func(s *Settings) { s.PaperLeft = max(0, s.PaperLeft-sheets) })
		return err
	}
}
