package relay

import (
	"context"
	"errors"
	"fmt"

	"go.wdy.de/nago/application/permission"
)

// CompletePairing schickt den Code, den der Besitzer an der Box eingetippt
// hat. Stimmt er, speichert die Box Adresse und Token selbst – das Token
// geht nicht durch die Oberfläche und wird nirgends angezeigt.
type CompletePairing func(subject permission.Auditable, ctx context.Context, p Pairing, code string) (PairingOutcome, error)

// Credentials speichert die Zugangsdaten zum Upload-Dienst und die Adresse
// des Kontos, mit dem gekoppelt wurde.
type Credentials func(url, token, account string) error

// NewCompletePairing erzeugt den Anwendungsfall.
func NewCompletePairing(save Credentials) CompletePairing {
	return func(subject permission.Auditable, ctx context.Context, p Pairing, code string) (PairingOutcome, error) {
		if err := subject.Audit(PermCompletePair); err != nil {
			return "", err
		}

		c, err := NewClient(Options{URL: p.URL})
		if err != nil {
			return "", err
		}

		outcome, token, err := c.ConfirmPairing(ctx, p.ID, code)
		if err != nil {
			return "", err
		}

		if outcome != PairingPaired {
			return outcome, nil
		}

		if token == "" {
			return "", errors.New("photoupld confirmed the pairing without a token")
		}

		if err := save(p.URL, token, p.Mail); err != nil {
			return "", fmt.Errorf("cannot save credentials: %w", err)
		}

		return PairingPaired, nil
	}
}
