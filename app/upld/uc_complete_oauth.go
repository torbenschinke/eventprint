package upld

// CompleteOAuth nimmt den Code aus dem Rücksprung des Fremddienstes entgegen
// und hält ihn für die Fotobox bereit.
//
// Wie [StartOAuth] ohne Subjekt: Den Rücksprung löst der Browser des
// Besitzers aus, nicht ein angemeldeter Nutzer. Angenommen wird ein Code nur
// für einen Vorgang, den eine Box zuvor angemeldet hat.
type CompleteOAuth func(state OAuthState, code string) error

// NewCompleteOAuth erzeugt den [CompleteOAuth] Anwendungsfall.
func NewCompleteOAuth(registry *OAuthRegistry) CompleteOAuth {
	return func(state OAuthState, code string) error {
		return registry.Complete(state, code)
	}
}
