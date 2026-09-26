package lightroom

import (
	"context"
	"errors"
	"net/url"

	"go.wdy.de/nago/application/permission"
)

// AwaitConnect fragt einmal beim Relay nach, ob das Telefon die Anmeldung
// abgeschlossen hat, und übernimmt sie gegebenenfalls.
//
// Eine einzelne Abfrage und keine Schleife: Die Oberfläche ruft den
// Anwendungsfall alle paar Sekunden auf und behält so die Kontrolle darüber,
// wann das Warten endet – etwa wenn der Besitzer den Dialog schließt.
type AwaitConnect func(subject permission.Auditable, ctx context.Context, state string) (Status, error)

// NewAwaitConnect bindet die Abholung an den Client.
func NewAwaitConnect(client *Client) AwaitConnect {
	return func(subject permission.Auditable, ctx context.Context, state string) (Status, error) {
		if err := subject.Audit(PermAwaitConnect); err != nil {
			return StatusPending, err
		}

		cfg := client.cfg()
		if err := cfg.requireOAuth(); err != nil {
			return StatusPending, err
		}

		if !validState(state) {
			return StatusPending, errors.New("ungültige Kennung des Anmeldevorgangs")
		}

		code, found, err := pollRelay(ctx, cfg, state)
		if err != nil {
			return StatusPending, err
		}

		if !found {
			return StatusExpired, nil
		}

		if code == "" {
			return StatusPending, nil
		}

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)

		tokens, err := exchangeTokens(ctx, cfg, client.now(), form)
		if err != nil {
			return StatusPending, err
		}

		// Unter der Sperre, damit eine gleichzeitig laufende Erneuerung der
		// alten Anmeldung die neuen Schlüssel nicht überschreibt.
		client.tokenMu.Lock()
		defer client.tokenMu.Unlock()

		if err := client.store.Save(tokens); err != nil {
			return StatusPending, err
		}

		// Eine neue Anmeldung kann ein anderes Konto sein; Katalog und Name
		// der alten dürfen nicht stehen bleiben.
		client.forget()

		return StatusConnected, nil
	}
}
