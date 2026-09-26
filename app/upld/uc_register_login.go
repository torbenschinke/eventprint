package upld

import "go.wdy.de/nago/auth"

// RegisterOAuth meldet einen Anmeldevorgang der anfragenden Fotobox an.
//
// authorizeURL ist die Anmeldeseite des Fremddienstes, auf die das Handy des
// Besitzers später weitergeleitet wird. Die Box baut sie selbst, denn nur sie
// kennt Client-ID, Scopes und PKCE-Challenge; das Relais prüft lediglich, dass
// sie wirklich zu Adobe führt.
type RegisterOAuth func(subject auth.Subject, state OAuthState, authorizeURL string) error

// NewRegisterOAuth erzeugt den [RegisterOAuth] Anwendungsfall.
func NewRegisterOAuth(registry *OAuthRegistry) RegisterOAuth {
	return func(subject auth.Subject, state OAuthState, authorizeURL string) error {
		if err := subject.Audit(PermRegisterOAuth); err != nil {
			return err
		}

		return registry.Register(tokenOf(subject), state, authorizeURL)
	}
}
