package upld

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testState     OAuthState = "0123456789abcdef0123456789abcdef"
	testAuthorize            = "https://ims-na1.adobelogin.com/ims/authorize/v2?client_id=x&state=0123456789abcdef0123456789abcdef"
)

// clock ist eine Uhr, die nur weiterläuft, wenn der Test es will.
type clock struct{ now time.Time }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func newTestOAuthRegistry() (*OAuthRegistry, *clock) {
	c := &clock{now: time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)}
	r := NewOAuthRegistry()
	r.now = c.Now

	return r, c
}

func TestOAuthFlowDeliversCodeExactlyOnce(t *testing.T) {
	r, _ := newTestOAuthRegistry()

	if err := r.Register("box", testState, testAuthorize); err != nil {
		t.Fatalf("Register: %v", err)
	}

	target, err := r.AuthorizeURL(testState)
	if err != nil || target != testAuthorize {
		t.Fatalf("AuthorizeURL = %q, %v", target, err)
	}

	// Solange der Rücksprung aussteht, ist der Code leer – kein Fehler.
	if code, err := r.Collect("box", testState); err != nil || code != "" {
		t.Fatalf("Collect vor dem Rücksprung = %q, %v", code, err)
	}

	if err := r.Complete(testState, "der-code"); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Ein Neuladen der Rücksprungseite bringt denselben Code noch einmal.
	if err := r.Complete(testState, "der-code"); err != nil {
		t.Fatalf("Complete (Neuladen): %v", err)
	}

	// Einen anderen Code darf niemand mehr unterschieben.
	if err := r.Complete(testState, "fremder-code"); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("Complete mit anderem Code: %v, erwartet ErrOAuthUnknown", err)
	}

	// Nach dem Rücksprung führt der Start-Link nirgends mehr hin.
	if _, err := r.AuthorizeURL(testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("AuthorizeURL nach Abschluss: %v, erwartet ErrOAuthUnknown", err)
	}

	code, err := r.Collect("box", testState)
	if err != nil || code != "der-code" {
		t.Fatalf("Collect = %q, %v", code, err)
	}

	if _, err := r.Collect("box", testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("zweites Collect: %v, erwartet ErrOAuthUnknown", err)
	}
}

func TestOAuthForeignTokenCannotCollect(t *testing.T) {
	r, _ := newTestOAuthRegistry()

	if err := r.Register("box-a", testState, testAuthorize); err != nil {
		t.Fatal(err)
	}

	if err := r.Complete(testState, "der-code"); err != nil {
		t.Fatal(err)
	}

	if _, err := r.Collect("box-b", testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("fremde Box: %v, erwartet ErrOAuthUnknown", err)
	}

	// Eine fremde Box darf den Vorgang auch nicht umlenken.
	if err := r.Register("box-b", testState, testAuthorize); !errors.Is(err, ErrOAuthInvalid) {
		t.Fatalf("fremde Box meldet denselben state an: %v, erwartet ErrOAuthInvalid", err)
	}

	// Der fehlgeschlagene Versuch hat der eigentlichen Box nichts genommen.
	if code, err := r.Collect("box-a", testState); err != nil || code != "der-code" {
		t.Fatalf("Collect der eigenen Box = %q, %v", code, err)
	}
}

func TestOAuthFlowExpires(t *testing.T) {
	r, c := newTestOAuthRegistry()

	if err := r.Register("box", testState, testAuthorize); err != nil {
		t.Fatal(err)
	}

	// Abfragen verlängern nichts: Es zählt das Alter, nicht der Leerlauf.
	c.Advance(OAuthTTL - time.Second)
	if _, err := r.Collect("box", testState); err != nil {
		t.Fatalf("Collect kurz vor Ablauf: %v", err)
	}

	c.Advance(time.Second)

	if _, err := r.AuthorizeURL(testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("AuthorizeURL nach Ablauf: %v", err)
	}

	if err := r.Complete(testState, "spaet"); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("Complete nach Ablauf: %v", err)
	}

	if _, err := r.Collect("box", testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("Collect nach Ablauf: %v", err)
	}
}

func TestOAuthPurgeExpiredFreesMemory(t *testing.T) {
	r, c := newTestOAuthRegistry()

	if err := r.Register("box", testState, testAuthorize); err != nil {
		t.Fatal(err)
	}

	c.Advance(OAuthTTL)
	r.PurgeExpired()

	if len(r.flows) != 0 {
		t.Fatalf("%d abgelaufene Vorgänge liegen noch im Speicher", len(r.flows))
	}
}

func TestOAuthRegisterRepeatIsIdempotent(t *testing.T) {
	r, _ := newTestOAuthRegistry()

	if err := r.Register("box", testState, testAuthorize); err != nil {
		t.Fatal(err)
	}

	if err := r.Register("box", testState, testAuthorize+"&x=1"); err != nil {
		t.Fatalf("Wiederholung derselben Box: %v", err)
	}

	if target, _ := r.AuthorizeURL(testState); target != testAuthorize+"&x=1" {
		t.Fatalf("AuthorizeURL = %q", target)
	}
}

func TestOAuthLimitsFlowsPerBox(t *testing.T) {
	r, c := newTestOAuthRegistry()

	var states []OAuthState
	for i := range maxOAuthFlowsPerToken + 1 {
		state := OAuthState(strings.Repeat(string(rune('a'+i)), 32))
		states = append(states, state)

		if err := r.Register("box", state, testAuthorize); err != nil {
			t.Fatal(err)
		}

		c.Advance(time.Second)
	}

	if _, err := r.AuthorizeURL(states[0]); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatal("der älteste Vorgang wurde nicht verdrängt")
	}

	for _, state := range states[1:] {
		if _, err := r.AuthorizeURL(state); err != nil {
			t.Fatalf("Vorgang %s fehlt: %v", state, err)
		}
	}

	// Eine andere Box ist davon nicht betroffen.
	other := OAuthState(strings.Repeat("z", 32))
	if err := r.Register("box-b", other, testAuthorize); err != nil {
		t.Fatal(err)
	}

	if _, err := r.AuthorizeURL(states[1]); err != nil {
		t.Fatal("die Anmeldung einer anderen Box hat einen Vorgang verdrängt")
	}
}

func TestOAuthValidation(t *testing.T) {
	states := []struct {
		state OAuthState
		ok    bool
	}{
		{state: testState, ok: true},
		{state: "aB3_-aB3_-aB3_-a", ok: true},
		{state: OAuthState(strings.Repeat("a", 128)), ok: true},
		{state: "zu-kurz", ok: false},
		{state: OAuthState(strings.Repeat("a", 129)), ok: false},
		{state: "0123456789abcdef/", ok: false},
		{state: "0123456789abcdef 01", ok: false},
		{state: "0123456789abcdefä", ok: false},
	}

	for _, tt := range states {
		err := ValidateOAuthState(tt.state)
		if (err == nil) != tt.ok {
			t.Errorf("ValidateOAuthState(%q) = %v, erwartet ok=%v", tt.state, err, tt.ok)
		}
	}

	urls := []struct {
		url string
		ok  bool
	}{
		{url: testAuthorize, ok: true},
		{url: "https://IMS-NA1.AdobeLogin.com/ims/authorize/v2", ok: true},
		{url: "https://ims-na1.adobelogin.com:443/ims/authorize/v2", ok: true},
		{url: "http://ims-na1.adobelogin.com/ims/authorize/v2", ok: false},
		{url: "https://adobelogin.com/ims/authorize/v2", ok: false},
		{url: "https://.adobelogin.com/", ok: false},
		{url: "https://evil.example/?x=.adobelogin.com", ok: false},
		{url: "https://evil-adobelogin.com/", ok: false},
		{url: "https://adobelogin.com.evil.example/", ok: false},
		{url: "https://ims-na1.adobelogin.com@evil.example/", ok: false},
		{url: "https://user@ims-na1.adobelogin.com/", ok: false},
		{url: "https://ims-na1.adobelogin.com:8443/", ok: false},
		{url: "//ims-na1.adobelogin.com/", ok: false},
		{url: "javascript:alert(1)//.adobelogin.com", ok: false},
		{url: "", ok: false},
		{url: "https://ims-na1.adobelogin.com/?" + strings.Repeat("a", maxAuthorizeURLLen), ok: false},
	}

	for _, tt := range urls {
		err := validateAuthorizeURL(tt.url)
		if (err == nil) != tt.ok {
			t.Errorf("validateAuthorizeURL(%q) = %v, erwartet ok=%v", tt.url, err, tt.ok)
		}

		if err != nil && !errors.Is(err, ErrOAuthInvalid) {
			t.Errorf("validateAuthorizeURL(%q) meldet %v statt ErrOAuthInvalid", tt.url, err)
		}
	}

	r, _ := newTestOAuthRegistry()
	if err := r.Register("box", testState, "https://evil.example/"); !errors.Is(err, ErrOAuthInvalid) {
		t.Fatalf("Register mit fremdem Ziel: %v, erwartet ErrOAuthInvalid", err)
	}

	if err := r.Register("box", testState, testAuthorize); err != nil {
		t.Fatal(err)
	}

	for _, code := range []string{"", "mit leerzeichen", "zeilen\numbruch", strings.Repeat("a", maxOAuthCodeLen+1)} {
		if err := r.Complete(testState, code); !errors.Is(err, ErrOAuthInvalid) {
			t.Errorf("Complete(%q) = %v, erwartet ErrOAuthInvalid", code, err)
		}
	}

	// Ein unbekannter oder ungültiger state verrät nichts über seine Form.
	if err := r.Complete("kaputt", "code"); !errors.Is(err, ErrOAuthUnknown) {
		t.Errorf("Complete mit kaputtem state: %v, erwartet ErrOAuthUnknown", err)
	}
}

// TestOAuthUseCasesSeparateBoxes prüft die Anwendungsfälle von der Anmeldung
// durch die Box bis zur Abholung – mit zwei Boxen, die sich nicht sehen
// dürfen.
func TestOAuthUseCasesSeparateBoxes(t *testing.T) {
	uc, _, _ := newTestUseCases(t)

	a, b := newBox("box-a"), newBox("box-b")

	if err := uc.RegisterOAuth(a, testState, testAuthorize); err != nil {
		t.Fatalf("RegisterOAuth: %v", err)
	}

	target, err := uc.StartOAuth(testState)
	if err != nil || target != testAuthorize {
		t.Fatalf("StartOAuth = %q, %v", target, err)
	}

	if err := uc.CompleteOAuth(testState, "der-code"); err != nil {
		t.Fatalf("CompleteOAuth: %v", err)
	}

	if _, err := uc.CollectOAuth(b, testState); !errors.Is(err, ErrOAuthUnknown) {
		t.Fatalf("CollectOAuth der fremden Box: %v, erwartet ErrOAuthUnknown", err)
	}

	code, err := uc.CollectOAuth(a, testState)
	if err != nil || code != "der-code" {
		t.Fatalf("CollectOAuth = %q, %v", code, err)
	}
}
