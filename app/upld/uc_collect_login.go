package upld

import "go.wdy.de/nago/auth"

// CollectLogin holt den Code eines Anmeldevorgangs für die anfragende
// Fotobox ab.
//
// Solange der Besitzer die Anmeldung am Handy noch nicht abgeschlossen hat,
// ist der Code leer. Danach wird er genau einmal ausgeliefert und ist
// anschließend verschwunden. Lesen darf ihn nur die Box, die den Vorgang
// angemeldet hat – sonst könnte eine fremde Box das Konto übernehmen.
type CollectLogin func(subject auth.Subject, state OAuthState) (code string, err error)

// NewCollectLogin erzeugt den [CollectLogin] Anwendungsfall.
func NewCollectLogin(registry *OAuthRegistry) CollectLogin {
	return func(subject auth.Subject, state OAuthState) (string, error) {
		if err := subject.Audit(PermCollectLogin); err != nil {
			return "", err
		}

		return registry.Collect(tokenOf(subject), state)
	}
}
