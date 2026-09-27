package pairing

import (
	"fmt"

	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
)

// UnpairBox trennt eine eigene Box: Ihr Zugangstoken wird gelöscht, und sie
// holt ab sofort keine Uploads mehr ab. Fremde Boxen trennt nur ein
// Administrator in der Token-Verwaltung.
type UnpairBox func(subject auth.Subject, id BoxID) error

// NewUnpairBox erzeugt den Anwendungsfall.
func NewUnpairBox(boxes Boxes, revoke Revoker) UnpairBox {
	return func(subject auth.Subject, id BoxID) error {
		if err := subject.Audit(PermUnpairBox); err != nil {
			return err
		}

		opt, err := boxes.FindByID(id)
		if err != nil {
			return err
		}

		// Eine fremde Box sieht aus wie eine, die es nicht gibt: Wer
		// Kennungen rät, erfährt nichts über die Boxen anderer.
		if opt.IsNone() || opt.Unwrap().Owner != string(subject.ID()) {
			return user.PermissionDeniedErr
		}

		if err := revoke(id); err != nil {
			return fmt.Errorf("cannot revoke token: %w", err)
		}

		return boxes.DeleteByID(id)
	}
}
