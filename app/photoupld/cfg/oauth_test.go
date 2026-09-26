package photoupld

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.wdy.de/nago/application/hapi"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/token"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/blob/mem"
	datajson "go.wdy.de/nago/pkg/data/json"
	oas "go.wdy.de/nago/pkg/oas/v31"

	"github.com/torbenschinke/eventprint/app/upld"
)

const (
	testState     = "0123456789abcdef0123456789abcdef"
	testAuthorize = "https://ims-na1.adobelogin.com/ims/authorize/v2?client_id=x&state=" + testState
)

// box ist eine Fotobox als Aufrufer; siehe app/upld.
type box struct {
	user.Subject
	id user.ID
}

func (b box) ID() user.ID { return b.id }

// stranger ist ein Token ohne die Rolle des Relais.
type stranger struct{ box }

func (stranger) HasPermission(permission.ID) bool { return false }
func (stranger) Audit(permission.ID) error        { return user.PermissionDeniedErr }

// muxHandlers sammelt alle Adressen in einem ServeMux.
type muxHandlers struct{ *http.ServeMux }

func (m muxHandlers) HandleMethod(method, pattern string, handler http.HandlerFunc) {
	m.HandleFunc(method+" "+pattern, handler)
}

func newOAuthServer(t *testing.T) http.Handler {
	t.Helper()

	images := image.NewUseCases(
		datajson.NewSloppyJSONRepository[image.SrcSet, image.ID](mem.NewBlobStore("img.set")),
		mem.NewBlobStore("img.blob"),
	)
	uc := upld.NewUseCases(upld.NewRegistry(nil), upld.NewOAuthRegistry(), images)

	mux := muxHandlers{http.NewServeMux()}
	api := hapi.NewAPI(&oas.OpenAPI{Paths: oas.Paths{}}, hapi.Options{RegisterHandler: mux.HandleMethod})

	authenticate := token.AuthenticateSubject(func(plaintext token.Plaintext) (auth.Subject, error) {
		switch plaintext {
		case "box-a", "box-b":
			return box{Subject: user.SU(), id: user.ID(plaintext)}, nil
		default:
			return stranger{box{Subject: user.SU(), id: user.ID(plaintext)}}, nil
		}
	})

	ConfigureOAuth(api, mux, authenticate, uc, func(state upld.OAuthState) string {
		return "https://relay.example" + OAuthStartPath + "?s=" + string(state)
	})

	return mux
}

func do(t *testing.T, srv http.Handler, method, target, bearer, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}

	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec
}

func register(t *testing.T, srv http.Handler, bearer, state, authorize string) *httptest.ResponseRecorder {
	t.Helper()

	body, _ := json.Marshal(OAuthRegisterRequest{State: state, AuthorizeURL: authorize})

	return do(t, srv, http.MethodPost, OAuthAPIPath, bearer, string(body))
}

func collect(t *testing.T, srv http.Handler, bearer string) (int, string) {
	t.Helper()

	rec := do(t, srv, http.MethodGet, OAuthAPIPath+"?state="+testState, bearer, "")
	if rec.Code != http.StatusOK {
		return rec.Code, ""
	}

	var res OAuthCollectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("Antwort ist kein JSON: %v: %s", err, rec.Body.String())
	}

	return rec.Code, res.Code
}

// TestOAuthRelayRoundTrip spielt die Anmeldung einmal vollständig durch:
// Box meldet an, Handy folgt dem Link, Adobe springt zurück, Box holt ab.
func TestOAuthRelayRoundTrip(t *testing.T) {
	srv := newOAuthServer(t)

	// Unbekannt, bevor die Box sich meldet.
	if code, _ := collect(t, srv, "box-a"); code != http.StatusNotFound {
		t.Fatalf("Abholung vor der Anmeldung: %d, erwartet 404", code)
	}

	rec := register(t, srv, "box-a", testState, testAuthorize)
	if rec.Code != http.StatusOK {
		t.Fatalf("Anmeldung: %d %s", rec.Code, rec.Body.String())
	}

	var reg OAuthRegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &reg); err != nil {
		t.Fatal(err)
	}

	if want := "https://relay.example/oauth/adobe/start?s=" + testState; reg.StartURL != want {
		t.Fatalf("startUrl = %q, erwartet %q", reg.StartURL, want)
	}

	// Solange das Handy nicht zurück ist: leerer Code.
	if status, code := collect(t, srv, "box-a"); status != http.StatusOK || code != "" {
		t.Fatalf("Abholung vor dem Rücksprung: %d %q", status, code)
	}

	rec = do(t, srv, http.MethodGet, OAuthStartPath+"?s="+testState, "", "")
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != testAuthorize {
		t.Fatalf("Start: %d nach %q", rec.Code, rec.Header().Get("Location"))
	}

	rec = do(t, srv, http.MethodGet, OAuthCallbackPath+"?code=der-code&state="+testState, "", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Verbunden!") {
		t.Fatalf("Rücksprung: %d %s", rec.Code, rec.Body.String())
	}

	// Eine fremde Box bekommt den Code nicht zu sehen.
	if status, _ := collect(t, srv, "box-b"); status != http.StatusNotFound {
		t.Fatalf("Abholung durch fremde Box: %d, erwartet 404", status)
	}

	// Ein Token ohne Rolle ebenso wenig.
	if status, _ := collect(t, srv, "unbekannt"); status != http.StatusForbidden {
		t.Fatalf("Abholung ohne Rolle: %d, erwartet 403", status)
	}

	if status, code := collect(t, srv, "box-a"); status != http.StatusOK || code != "der-code" {
		t.Fatalf("Abholung: %d %q", status, code)
	}

	// Genau einmal.
	if status, _ := collect(t, srv, "box-a"); status != http.StatusNotFound {
		t.Fatalf("zweite Abholung: %d, erwartet 404", status)
	}

	// Der Start-Link führt danach nirgends mehr hin.
	rec = do(t, srv, http.MethodGet, OAuthStartPath+"?s="+testState, "", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "Dieser Link ist abgelaufen.") {
		t.Fatalf("Start nach Abschluss: %d %s", rec.Code, rec.Body.String())
	}
}

func TestOAuthRelayRejectsBadRegistrations(t *testing.T) {
	srv := newOAuthServer(t)

	for _, tt := range []struct{ state, authorize string }{
		{state: testState, authorize: "https://evil.example/login"},
		{state: testState, authorize: "http://ims-na1.adobelogin.com/ims/authorize/v2"},
		{state: "kurz", authorize: testAuthorize},
		{state: testState + "/..", authorize: testAuthorize},
	} {
		if rec := register(t, srv, "box-a", tt.state, tt.authorize); rec.Code != http.StatusBadRequest {
			t.Errorf("Anmeldung %q → %q: %d, erwartet 400", tt.state, tt.authorize, rec.Code)
		}
	}

	if rec := register(t, srv, "unbekannt", testState, testAuthorize); rec.Code == http.StatusOK {
		t.Error("ein Token ohne Rolle konnte eine Anmeldung vermitteln")
	}

	// Keine der abgewiesenen Anmeldungen darf eine Weiterleitung hinterlassen.
	if rec := do(t, srv, http.MethodGet, OAuthStartPath+"?s="+testState, "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("Start nach abgewiesener Anmeldung: %d, erwartet 404", rec.Code)
	}
}

func TestOAuthCallbackFailures(t *testing.T) {
	srv := newOAuthServer(t)

	// Unbekannter Vorgang.
	rec := do(t, srv, http.MethodGet, OAuthCallbackPath+"?code=x&state="+testState, "", "")
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "abgelaufen") {
		t.Fatalf("Rücksprung ohne Anmeldung: %d %s", rec.Code, rec.Body.String())
	}

	if rec := register(t, srv, "box-a", testState, testAuthorize); rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}

	// Adobe meldet einen Abbruch. Der Vorgang bleibt offen.
	rec = do(t, srv, http.MethodGet, OAuthCallbackPath+"?error=access_denied&state="+testState, "", "")
	if rec.Code != http.StatusBadRequest || strings.Contains(rec.Body.String(), "Verbunden!") {
		t.Fatalf("Rücksprung mit error: %d %s", rec.Code, rec.Body.String())
	}

	if status, code := collect(t, srv, "box-a"); status != http.StatusOK || code != "" {
		t.Fatalf("Abholung nach Abbruch: %d %q", status, code)
	}

	// Ohne Code gibt es nichts zu verbinden.
	rec = do(t, srv, http.MethodGet, OAuthCallbackPath+"?state="+testState, "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Rücksprung ohne Code: %d", rec.Code)
	}

	// Die Seiten dürfen state und Code nicht weitertragen.
	if got := rec.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
}
