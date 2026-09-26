package photoupld

import (
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

	"go.wdy.de/nago/application/hapi"
	"go.wdy.de/nago/application/token"
	"go.wdy.de/nago/auth"
	oas "go.wdy.de/nago/pkg/oas/v31"

	"github.com/torbenschinke/eventprint/app/upld"
)

const (
	OAuthAPIPath      = "/api/v1/oauth/adobe"
	OAuthStartPath    = "/oauth/adobe/start"
	OAuthCallbackPath = "/oauth/adobe/callback"
)

type OAuthRegisterRequest struct {
	State        string `json:"state"`
	AuthorizeURL string `json:"authorizeUrl"`
}

type OAuthRegisterResponse struct {
	StartURL string `json:"startUrl"`
}

type OAuthCollectResponse struct {
	Code string `json:"code"`
}

type oauthRegistration struct {
	authenticated
	State        upld.OAuthState
	AuthorizeURL string
}

// Handlers ist die Stelle, an der das Relais HTTP-Adressen annimmt.
//
// Ein Interface statt des Configurators, damit sich die Handler ohne
// laufenden Server prüfen lassen.
type Handlers interface {
	HandleMethod(method string, pattern string, handler http.HandlerFunc)
}

// ConfigureOAuth hängt die Anmeldung bei Adobe an HTTP-Adressen.
//
// Die Fotobox ist aus dem Internet nicht erreichbar und kann den Rücksprung
// von Adobe nicht empfangen. Das Handy des Besitzers landet deshalb hier; die
// Box meldet den Vorgang vorher an und holt den Code danach ab.
func ConfigureOAuth(api *hapi.API, mux Handlers, authenticate token.AuthenticateSubject, uc upld.UseCases, startURL func(upld.OAuthState) string) {
	registration := func(dst *oauthRegistration, subject auth.Subject) error {
		dst.Subject = subject
		warnAboutToken(subject)

		return nil
	}

	hapi.Post[oauthRegistration](api, hapi.Operation{Path: OAuthAPIPath, Summary: "Anmeldung bei Adobe vermitteln"}).
		Request(
			hapi.BearerAuth(authenticate, registration),
			hapi.JSONFromBody(func(dst *oauthRegistration, model OAuthRegisterRequest) error {
				dst.State = upld.OAuthState(model.State)
				dst.AuthorizeURL = model.AuthorizeURL

				return nil
			}),
		).
		Response(hapi.ToJSON[oauthRegistration, OAuthRegisterResponse](func(in oauthRegistration) (OAuthRegisterResponse, error) {
			if err := uc.RegisterOAuth(in.Subject, in.State, in.AuthorizeURL); err != nil {
				return OAuthRegisterResponse{}, err
			}

			return OAuthRegisterResponse{StartURL: startURL(in.State)}, nil
		}))

	// Die Abholung läuft an hapi vorbei. hapi beantwortet jeden Fehler eines
	// Anwendungsfalls mit 400; die Box muss aber "gibt es nicht (mehr)" von
	// einer vorübergehenden Störung unterscheiden, um die Anmeldung
	// abzubrechen statt endlos weiterzufragen. Das verlangt ein 404.
	mux.HandleMethod(http.MethodGet, OAuthAPIPath, collectOAuthHandler(authenticate, uc.CollectOAuth))
	hapi.Doc(api, func(doc *oas.OpenAPI) {
		if item, ok := doc.Paths[OAuthAPIPath]; ok && item != nil {
			item.Get = &oas.Operation{
				Summary:     "Code einer Adobe-Anmeldung abholen",
				Description: "Query-Parameter state. Liefert {\"code\":\"\"}, solange die Anmeldung aussteht, und den Code genau einmal, sobald sie abgeschlossen ist. 404, wenn der Vorgang unbekannt, abgelaufen oder bereits abgeholt ist.",
			}
		}
	})

	mux.HandleMethod(http.MethodGet, OAuthStartPath, startOAuthHandler(uc.StartOAuth))
	mux.HandleMethod(http.MethodGet, OAuthCallbackPath, completeOAuthHandler(uc.CompleteOAuth))
}

func collectOAuthHandler(authenticate token.AuthenticateSubject, collect upld.CollectOAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		subject, ok := bearerSubject(w, r, authenticate)
		if !ok {
			return
		}

		warnAboutToken(subject)

		code, err := collect(subject, upld.OAuthState(r.URL.Query().Get("state")))
		switch {
		case err == nil:
		case errors.Is(err, upld.ErrOAuthUnknown):
			http.Error(w, "unknown or expired state", http.StatusNotFound)
			return
		case errors.Is(err, upld.ErrOAuthInvalid):
			http.Error(w, "invalid state", http.StatusBadRequest)
			return
		default:
			// Was übrig bleibt, stammt aus der Rechteprüfung: fehlende Rolle,
			// unbekanntes Token oder ungültiges Konto. warnAboutToken hat den
			// Grund bereits ins Protokoll geschrieben.
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if err := json.NewEncoder(w).Encode(OAuthCollectResponse{Code: code}); err != nil {
			slog.Error("photoupld: Antwort nicht schreibbar", "err", err)
		}
	}
}

// bearerSubject meldet den Aufrufer so an, wie hapi.BearerAuth es tut.
//
// Ohne Kopfzeile entsteht ein anonymes Subjekt, das am Anwendungsfall
// scheitert – dieselbe Fehlerkette wie bei allen anderen Adressen der Box.
func bearerSubject(w http.ResponseWriter, r *http.Request, authenticate token.AuthenticateSubject) (auth.Subject, bool) {
	header := r.Header.Get("Authorization")

	var plaintext token.Plaintext
	if header != "" {
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			http.Error(w, "invalid auth header format", http.StatusUnauthorized)
			return nil, false
		}

		plaintext = token.Plaintext(strings.TrimPrefix(header, prefix))
	}

	subject, err := authenticate(plaintext)
	if err != nil || subject == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}

	return subject, true
}

func startOAuthHandler(start upld.StartOAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noLeaks(w)

		target, err := start(upld.OAuthState(r.URL.Query().Get("s")))
		if err != nil {
			renderOAuthPage(w, http.StatusNotFound, oauthExpired)
			return
		}

		http.Redirect(w, r, target, http.StatusFound)
	}
}

func completeOAuthHandler(complete upld.CompleteOAuth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		noLeaks(w)

		query := r.URL.Query()

		// Adobe meldet Abbruch oder Ablehnung über error. Der Vorgang bleibt
		// dabei offen: Wer sich am Handy nur vertippt hat, kann über denselben
		// QR-Code einfach noch einmal ansetzen.
		if query.Get("error") != "" {
			slog.Info("photoupld: Adobe hat die Anmeldung nicht bestätigt", "error", query.Get("error"))
			renderOAuthPage(w, http.StatusBadRequest, oauthDenied)
			return
		}

		err := complete(upld.OAuthState(query.Get("state")), query.Get("code"))
		switch {
		case err == nil:
			renderOAuthPage(w, http.StatusOK, oauthConnected)
		case errors.Is(err, upld.ErrOAuthUnknown):
			renderOAuthPage(w, http.StatusNotFound, oauthExpired)
		default:
			slog.Warn("photoupld: Rücksprung von Adobe abgewiesen", "err", err)
			renderOAuthPage(w, http.StatusBadRequest, oauthDenied)
		}
	}
}

// noLeaks hält Start- und Rücksprungadresse aus Caches und Referer-Kopfzeilen
// heraus. Beide tragen den state, die Rücksprungadresse zusätzlich den Code.
func noLeaks(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
}

type oauthPage struct {
	Title string
	Text  string
}

var (
	oauthConnected = oauthPage{Title: "Verbunden!", Text: "Du kannst zur Box zurückkehren."}
	oauthExpired   = oauthPage{Title: "Dieser Link ist abgelaufen.", Text: "Bitte an der Box neu starten."}
	oauthDenied    = oauthPage{Title: "Nicht verbunden", Text: "Die Anmeldung wurde abgebrochen. Bitte an der Box neu starten."}
)

// oauthPageTemplate ist bewusst eine schlichte HTML-Seite und keine
// Nago-Seite.
//
// Das Handy bleibt hier nur einen Augenblick und soll danach zur Box
// zurückkehren. Eine Nago-Seite bräuchte dafür JavaScript und eine
// Websocket-Verbindung – zwei Dinge mehr, die in einem Hotel-WLAN scheitern
// können, ohne dass der Besitzer je erfährt, ob die Verbindung geklappt hat.
var oauthPageTemplate = template.Must(template.New("oauth").Parse(`<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<style>
:root { color-scheme: light dark; }
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
       font-family: system-ui, -apple-system, "Segoe UI", Roboto, sans-serif; text-align: center;
       background: #fff; color: #1b1b1f; }
main { padding: 2rem; max-width: 28rem; }
h1 { font-size: 2.25rem; font-weight: 400; margin: 0 0 1rem; }
p { font-size: 1rem; line-height: 1.5; margin: 0; }
@media (prefers-color-scheme: dark) { body { background: #1b1b1f; color: #e4e1e6; } }
</style>
</head>
<body><main><h1>{{.Title}}</h1><p>{{.Text}}</p></main></body>
</html>
`))

func renderOAuthPage(w http.ResponseWriter, status int, page oauthPage) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := oauthPageTemplate.Execute(w, page); err != nil {
		slog.Error("photoupld: Seite nicht schreibbar", "err", err)
	}
}
