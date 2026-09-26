package upld

// StartOAuth liefert die Anmeldeseite, auf die das Handy des Besitzers
// weitergeleitet wird.
//
// Der Anwendungsfall hat bewusst kein Subjekt: Das Handy hat den Link gerade
// per QR-Code von der Box abgelesen und ist am Relais niemand. Die
// Berechtigung steckt allein in der Kenntnis des zufälligen state – genau wie
// bei der Upload-Seite in der Kenntnis der Upload-Kennung.
type StartOAuth func(state OAuthState) (authorizeURL string, err error)

// NewStartOAuth erzeugt den [StartOAuth] Anwendungsfall.
func NewStartOAuth(registry *OAuthRegistry) StartOAuth {
	return func(state OAuthState) (string, error) {
		return registry.AuthorizeURL(state)
	}
}
