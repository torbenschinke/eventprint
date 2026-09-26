package upld

import "go.wdy.de/nago/auth"

// CollectOAuth holt den Code eines Anmeldevorgangs für die anfragende
// Fotobox ab.
//
// Solange der Besitzer die Anmeldung am Handy noch nicht abgeschlossen hat,
// ist der Code leer. Danach wird er genau einmal ausgeliefert und ist
// anschließend verschwunden. Lesen darf ihn nur die Box, die den Vorgang
// angemeldet hat – sonst könnte eine fremde Box das Konto übernehmen.
type CollectOAuth func(subject auth.Subject, state OAuthState) (code string, err error)

// NewCollectOAuth erzeugt den [CollectOAuth] Anwendungsfall.
func NewCollectOAuth(registry *OAuthRegistry) CollectOAuth {
	return func(subject auth.Subject, state OAuthState) (string, error) {
		if err := subject.Audit(PermCollectOAuth); err != nil {
			return "", err
		}

		return registry.Collect(tokenOf(subject), state)
	}
}
