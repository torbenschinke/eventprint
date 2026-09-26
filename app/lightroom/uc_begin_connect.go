package lightroom

import (
	"context"

	"go.wdy.de/nago/application/permission"
)

// BeginConnect beginnt die Anmeldung bei Adobe und liefert die Adresse für
// den QR-Code.
//
// Die Box selbst hat keine öffentliche Adresse, an die Adobe nach der
// Anmeldung zurückleiten könnte. Deshalb meldet sie den Vorgang beim Relay an;
// der nimmt den Autorisierungscode entgegen, und [AwaitConnect] holt ihn dort
// ab.
type BeginConnect func(subject permission.Auditable, ctx context.Context) (Authorization, error)

// NewBeginConnect bindet den Verbindungsaufbau an den Client.
func NewBeginConnect(client *Client) BeginConnect {
	return func(subject permission.Auditable, ctx context.Context) (Authorization, error) {
		if err := subject.Audit(PermBeginConnect); err != nil {
			return Authorization{}, err
		}

		cfg := client.cfg()
		if err := cfg.requireOAuth(); err != nil {
			return Authorization{}, err
		}

		state, err := newState()
		if err != nil {
			return Authorization{}, err
		}

		start, err := registerAtRelay(ctx, cfg, state, authorizeURL(cfg, state))
		if err != nil {
			return Authorization{}, err
		}

		return Authorization{
			State:     state,
			StartURL:  start,
			ExpiresAt: client.now().Add(relayStateLifetime),
		}, nil
	}
}
