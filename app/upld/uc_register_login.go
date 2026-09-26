package upld

import "go.wdy.de/nago/auth"

// RegisterLogin meldet einen Anmeldevorgang der anfragenden Fotobox an.
//
// authorizeURL ist die Anmeldeseite des Fremddienstes, auf die das Handy des
// Besitzers später weitergeleitet wird. Die Box baut sie selbst, denn nur sie
// kennt Client-ID, Scopes und PKCE-Challenge; das Relais prüft lediglich, dass
// sie wirklich zu Adobe führt.
type RegisterLogin func(subject auth.Subject, state OAuthState, authorizeURL string) error

// NewRegisterLogin erzeugt den [RegisterLogin] Anwendungsfall.
func NewRegisterLogin(registry *OAuthRegistry) RegisterLogin {
	return func(subject auth.Subject, state OAuthState, authorizeURL string) error {
		if err := subject.Audit(PermRegisterLogin); err != nil {
			return err
		}

		return registry.Register(tokenOf(subject), state, authorizeURL)
	}
}
