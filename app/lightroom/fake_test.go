package lightroom_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/lightroom"
)

// allow ist ein Subjekt, das alles darf, und merkt sich die geprüften
// Berechtigungen.
type allow struct {
	mu      sync.Mutex
	audited []permission.ID
}

func (a *allow) Audit(id permission.ID) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.audited = append(a.audited, id)

	return nil
}

func (a *allow) HasPermission(permission.ID) bool { return true }

// deny ist ein Subjekt ohne jede Berechtigung.
type deny struct{}

var errDenied = errors.New("verweigert")

func (deny) Audit(permission.ID) error        { return errDenied }
func (deny) HasPermission(permission.ID) bool { return false }

// memStore ist eine Schlüsselablage im Speicher.
type memStore struct {
	mu     sync.Mutex
	tokens *lightroom.Tokens
}

func (s *memStore) Load() (lightroom.Tokens, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tokens == nil {
		return lightroom.Tokens{}, false, nil
	}

	return *s.tokens, true, nil
}

func (s *memStore) Save(t lightroom.Tokens) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens = &t

	return nil
}

func (s *memStore) Delete() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tokens = nil

	return nil
}

func (s *memStore) get() (lightroom.Tokens, bool) {
	t, ok, _ := s.Load()
	return t, ok
}

const (
	testClientID     = "client-123"
	testClientSecret = "secret-456"
	testRelayToken   = "relay-789"
	testCatalog      = "cat1"
)

// fakeAdobe spielt Relay, Adobe-Anmeldung und Lightroom-Schnittstelle in
// einem Server.
type fakeAdobe struct {
	t   *testing.T
	srv *httptest.Server

	mu sync.Mutex

	// Relay
	registered map[string]string // State → Anmeldeadresse
	codes      map[string]string // State → Code (leer = wartend)
	relayError map[string]string

	// Anmeldung
	validCodes     map[string]bool
	accessTokens   map[string]bool
	refreshTokens  map[string]bool
	refreshStatus  int  // != 0: Erneuerung scheitert mit diesem Status
	rotateRefresh  bool // Erneuerung liefert einen neuen Refresh-Token
	issued         int
	tokenCalls     int
	refreshCalls   int
	lastTokenForm  map[string]string
	requestCounter int

	// Lightroom
	catalogCalls int
	renditions   map[string]map[string][]byte // Asset → Stufe → Bytes
	fileNames    map[string]string
	lastAPIKey   string
}

func newFakeAdobe(t *testing.T) *fakeAdobe {
	f := &fakeAdobe{
		t:             t,
		registered:    map[string]string{},
		codes:         map[string]string{},
		relayError:    map[string]string{},
		validCodes:    map[string]bool{},
		accessTokens:  map[string]bool{},
		refreshTokens: map[string]bool{},
		renditions:    map[string]map[string][]byte{},
		fileNames:     map[string]string{},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/oauth/adobe", f.relayRegister)
	mux.HandleFunc("GET /api/v1/oauth/adobe", f.relayPoll)
	mux.HandleFunc("POST /ims/token/v3", f.token)
	mux.HandleFunc("GET /v2/catalog", f.lr(f.catalog))
	mux.HandleFunc("GET /v2/account", f.lr(f.account))
	mux.HandleFunc("GET /v2/catalogs/{cid}/albums", f.lr(f.albums))
	mux.HandleFunc("GET /v2/catalogs/{cid}/albums/{aid}/assets", f.lr(f.albumAssets))
	mux.HandleFunc("GET /v2/catalogs/{cid}/assets", f.lr(f.allAssets))
	mux.HandleFunc("GET /v2/catalogs/{cid}/assets/{aid}", f.lr(f.asset))
	mux.HandleFunc("GET /v2/catalogs/{cid}/assets/{aid}/renditions/{kind}", f.lr(f.rendition))

	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requestCounter++
		f.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.srv.Close)

	return f
}

func (f *fakeAdobe) config() lightroom.Config {
	return lightroom.Config{
		ClientID:     testClientID,
		ClientSecret: testClientSecret,
		RelayURL:     f.srv.URL + "/",
		RelayToken:   testRelayToken,
		IMSBaseURL:   f.srv.URL,
		APIBaseURL:   f.srv.URL,
		HTTPClient:   f.srv.Client(),
	}
}

func (f *fakeAdobe) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.requestCounter
}

// grant hinterlegt gültige Schlüssel, als hätte eine Anmeldung stattgefunden.
func (f *fakeAdobe) grant(store *memStore, expiresIn time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.issued++
	access := fmt.Sprintf("access-%d", f.issued)
	refresh := fmt.Sprintf("refresh-%d", f.issued)
	f.accessTokens[access] = true
	f.refreshTokens[refresh] = true

	_ = store.Save(lightroom.Tokens{AccessToken: access, RefreshToken: refresh, ExpiresAt: time.Now().Add(expiresIn)})
}

func (f *fakeAdobe) relayRegister(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+testRelayToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var in struct {
		State        string `json:"state"`
		AuthorizeURL string `json:"authorizeUrl"`
	}

	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.State == "" || in.AuthorizeURL == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	f.registered[in.State] = in.AuthorizeURL
	f.codes[in.State] = ""
	f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"startUrl": f.srv.URL + "/oauth/adobe/start?s=" + in.State})
}

func (f *fakeAdobe) relayPoll(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer "+testRelayToken {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	state := r.URL.Query().Get("state")

	f.mu.Lock()
	code, ok := f.codes[state]
	relayErr := f.relayError[state]
	f.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := map[string]string{"code": code}
	if relayErr != "" {
		resp["error"] = relayErr
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// complete spielt das Telefon: Die Anmeldung ist abgeschlossen, der Relay hat
// den Code.
func (f *fakeAdobe) complete(state string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	code := "code-for-" + state
	f.codes[state] = code
	f.validCodes[code] = true
}

func (f *fakeAdobe) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.tokenCalls++
	f.lastTokenForm = map[string]string{}

	for k := range r.PostForm {
		f.lastTokenForm[k] = r.PostForm.Get(k)
	}

	if r.PostForm.Get("client_id") != testClientID || r.PostForm.Get("client_secret") != testClientSecret {
		writeJSONError(w, http.StatusUnauthorized, "invalid_client")
		return
	}

	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		code := r.PostForm.Get("code")
		if !f.validCodes[code] {
			writeJSONError(w, http.StatusBadRequest, "invalid_grant")
			return
		}

		delete(f.validCodes, code)
	case "refresh_token":
		f.refreshCalls++

		if f.refreshStatus != 0 {
			writeJSONError(w, f.refreshStatus, "invalid_grant")
			return
		}

		if !f.refreshTokens[r.PostForm.Get("refresh_token")] {
			writeJSONError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
	default:
		writeJSONError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}

	f.issued++
	access := fmt.Sprintf("access-%d", f.issued)
	f.accessTokens[access] = true

	resp := map[string]any{"access_token": access, "token_type": "bearer", "expires_in": 86399}

	if r.PostForm.Get("grant_type") == "authorization_code" || f.rotateRefresh {
		refresh := fmt.Sprintf("refresh-%d", f.issued)
		f.refreshTokens[refresh] = true
		resp["refresh_token"] = refresh
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// revokeAccess lässt Lightroom alle bisherigen Zugriffsschlüssel ablehnen.
func (f *fakeAdobe) revokeAccess() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.accessTokens = map[string]bool{}
}

// lr prüft die Kopfzeilen, die Lightroom für jede Anfrage verlangt.
func (f *fakeAdobe) lr(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastAPIKey = r.Header.Get("X-API-Key")
		valid := f.accessTokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
		f.mu.Unlock()

		if r.Header.Get("X-API-Key") != testClientID {
			http.Error(w, "missing api key", http.StatusForbidden)
			return
		}

		if !valid {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if cid := r.PathValue("cid"); cid != "" && cid != testCatalog {
			http.NotFound(w, r)
			return
		}

		next(w, r)
	}
}

// writeLR schreibt JSON mit dem Vorspann, den Lightroom voranstellt.
func writeLR(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte("while (1) {}\n"))
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakeAdobe) catalog(w http.ResponseWriter, _ *http.Request) {
	f.mu.Lock()
	f.catalogCalls++
	f.mu.Unlock()

	writeLR(w, map[string]any{"id": testCatalog, "type": "catalog", "payload": map[string]any{"name": "Lightroom"}})
}

func (f *fakeAdobe) account(w http.ResponseWriter, _ *http.Request) {
	writeLR(w, map[string]any{"id": "acc", "full_name": "Torben Schinke", "email": "t@example.de"})
}

// albums liefert drei Seiten: Die erste verweist relativ zu "base", die
// zweite mit absolutem Pfad.
func (f *fakeAdobe) albums(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("subtype") != "collection" {
		http.Error(w, "subtype fehlt", http.StatusBadRequest)
		return
	}

	base := f.srv.URL + "/v2/catalogs/" + testCatalog + "/"

	switch r.URL.Query().Get("page") {
	case "":
		writeLR(w, map[string]any{
			"base": base,
			"resources": []any{
				map[string]any{"id": "a-zoo", "subtype": "collection", "payload": map[string]any{"name": "Zoo", "cover": map[string]any{"id": "cover-1"}}},
				map[string]any{"id": "set-1", "subtype": "collection_set", "payload": map[string]any{"name": "Ordner"}},
				"kaputt",
			},
			"links": map[string]any{"next": map[string]any{"href": "albums?subtype=collection&page=2"}},
		})
	case "2":
		writeLR(w, map[string]any{
			"base": base,
			"resources": []any{
				map[string]any{"id": "a-hochzeit", "subtype": "collection", "payload": map[string]any{"name": "hochzeit"}},
			},
			"links": map[string]any{"next": map[string]any{"href": "/v2/catalogs/" + testCatalog + "/albums?subtype=collection&page=3"}},
		})
	case "3":
		writeLR(w, map[string]any{
			"base": base,
			"resources": []any{
				map[string]any{"id": "a-geburtstag", "subtype": "collection", "payload": map[string]any{"name": "Geburtstag"}},
			},
			"links": map[string]any{},
		})
	default:
		http.NotFound(w, r)
	}
}

func imageAsset(id, subtype string, payload map[string]any) map[string]any {
	return map[string]any{"id": id, "type": "asset", "subtype": subtype, "payload": payload}
}

func (f *fakeAdobe) albumAssets(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("embed") != "asset" {
		http.Error(w, "embed fehlt", http.StatusBadRequest)
		return
	}

	base := f.srv.URL + "/v2/catalogs/" + testCatalog + "/"
	aid := r.PathValue("aid")

	switch {
	case aid == "a-zoo" && r.URL.Query().Get("after") == "":
		writeLR(w, map[string]any{
			"base": base,
			"resources": []any{
				map[string]any{"id": "img-1", "asset": imageAsset("img-1", "image", map[string]any{
					"captureDate": "2024-06-01T14:30:00",
					"importSource": map[string]any{
						"fileName": "IMG_0001.CR3", "originalWidth": 6000, "originalHeight": 4000,
					},
					"develop": map[string]any{"croppedWidth": 3000, "croppedHeight": "2000"},
					"rating":  3,
				})},
				map[string]any{"id": "vid-1", "asset": imageAsset("vid-1", "video", map[string]any{})},
			},
			"links": map[string]any{"next": map[string]any{"href": "albums/a-zoo/assets?embed=asset&limit=100&after=img-1"}},
		})
	case aid == "a-zoo":
		writeLR(w, map[string]any{
			"base": base,
			"resources": []any{
				map[string]any{"id": "img-2", "asset": imageAsset("img-2", "image", map[string]any{
					"captureDate":  "2024-06-01T15:00:00.123Z",
					"importSource": map[string]any{"fileName": "DSC_2.jpg", "originalWidth": 4000, "originalHeight": 6000},
					"ratings":      map[string]any{"user-a": map[string]any{"rating": 4}, "user-b": map[string]any{"rating": 2}},
				})},
			},
			"links": map[string]any{},
		})
	case aid == "a-videos" && r.URL.Query().Get("after") == "":
		writeLR(w, map[string]any{
			"base":      base,
			"resources": []any{map[string]any{"id": "vid-2", "asset": imageAsset("vid-2", "video", nil)}},
			"links":     map[string]any{"next": map[string]any{"href": "albums/a-videos/assets?embed=asset&limit=100&after=vid-2"}},
		})
	case aid == "a-videos":
		writeLR(w, map[string]any{
			"base":      base,
			"resources": []any{map[string]any{"id": "img-3", "asset": imageAsset("img-3", "image", map[string]any{})}},
		})
	case aid == "a-evil":
		writeLR(w, map[string]any{
			"resources": []any{},
			"links":     map[string]any{"next": map[string]any{"href": "https://evil.example/v2/catalogs/cat1/x"}},
		})
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAdobe) allAssets(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("subtype") != "image" {
		http.Error(w, "subtype fehlt", http.StatusBadRequest)
		return
	}

	writeLR(w, map[string]any{
		"resources": []any{
			imageAsset("img-9", "image", map[string]any{
				"captureDate":  "0000-00-00T00:00:00",
				"importSource": map[string]any{"fileName": "IMG_9.HEIC", "originalWidth": 4032, "originalHeight": 3024},
			}),
		},
		"links": map[string]any{},
	})
}

func (f *fakeAdobe) asset(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	name, ok := f.fileNames[r.PathValue("aid")]
	f.mu.Unlock()

	if !ok {
		http.NotFound(w, r)
		return
	}

	writeLR(w, imageAsset(r.PathValue("aid"), "image", map[string]any{"importSource": map[string]any{"fileName": name}}))
}

func (f *fakeAdobe) rendition(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	data, ok := f.renditions[r.PathValue("aid")][r.PathValue("kind")]
	f.mu.Unlock()

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("while (1) {}\n{\"code\":1000,\"description\":\"Resource not found\"}"))

		return
	}

	w.Header().Set("Content-Type", "image/jpeg")
	_, _ = w.Write(data)
}
