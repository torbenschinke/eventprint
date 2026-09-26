package upld

// CompleteLogin nimmt den Code aus dem Rücksprung des Fremddienstes entgegen
// und hält ihn für die Fotobox bereit.
//
// Wie [StartLogin] ohne Subjekt: Den Rücksprung löst der Browser des
// Besitzers aus, nicht ein angemeldeter Nutzer. Angenommen wird ein Code nur
// für einen Vorgang, den eine Box zuvor angemeldet hat.
type CompleteLogin func(state OAuthState, code string) error

// NewCompleteLogin erzeugt den [CompleteLogin] Anwendungsfall.
func NewCompleteLogin(registry *OAuthRegistry) CompleteLogin {
	return func(state OAuthState, code string) error {
		return registry.Complete(state, code)
	}
}
