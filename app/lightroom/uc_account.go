package lightroom

import (
	"context"
	"errors"

	"go.wdy.de/nago/application/permission"
)

// Account meldet, ob die Box mit Lightroom verbunden ist und mit welchem
// Konto.
type Account func(subject permission.Auditable, ctx context.Context) (AccountInfo, error)

// NewAccount bindet die Auskunft an den Client.
//
// Scheitert nur die Abfrage des Namens, etwa weil das Funknetz gerade fehlt,
// gilt die Box weiterhin als verbunden: Die Schlüssel sind da, und ein
// Hinweis zur neuen Anmeldung wäre in diesem Fall falsch. Nur wenn Adobe die
// Anmeldung verworfen hat, ist die Verbindung weg.
func NewAccount(client *Client) Account {
	return func(subject permission.Auditable, ctx context.Context) (AccountInfo, error) {
		if err := subject.Audit(PermAccount); err != nil {
			return AccountInfo{}, err
		}

		_, ok, err := client.store.Load()
		if err != nil {
			return AccountInfo{}, err
		}

		if !ok {
			return AccountInfo{}, nil
		}

		name, err := client.account(ctx, client.cfg())
		if err != nil {
			var notConnected NotConnectedError
			if errors.As(err, &notConnected) {
				return AccountInfo{}, nil
			}

			return AccountInfo{Connected: true}, nil
		}

		return AccountInfo{Connected: true, Name: name}, nil
	}
}
