package upld

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthState ist die Kennung eines Anmeldevorgangs bei einem Fremddienst.
//
// Die Fotobox würfelt sie selbst aus und schickt sie im OAuth-Parameter state
// mit. Der Fremddienst gibt sie beim Rücksprung unverändert zurück; daran
// erkennt das Relais, für welche Box der Code bestimmt ist.
type OAuthState string

var (
	// ErrOAuthUnknown meldet einen Vorgang, den das Relais nicht (mehr) kennt.
	//
	// Unbekannt, abgelaufen, bereits abgeholt oder fremd sind von außen
	// bewusst nicht zu unterscheiden: Der Unterschied verriete, welche
	// Kennungen existieren.
	ErrOAuthUnknown = errors.New("unknown or expired oauth state")

	// ErrOAuthInvalid meldet eine Anfrage, die das Relais gar nicht erst
	// annimmt.
	ErrOAuthInvalid = errors.New("invalid oauth request")
)

const (
	// OAuthTTL ist die Lebensdauer eines Anmeldevorgangs, gerechnet ab seiner
	// Anmeldung durch die Fotobox.
	//
	// Anders als bei der Upload-Sitzung zählt hier das Alter und nicht der
	// letzte Zugriff: Die Box fragt während der Anmeldung laufend nach, ein
	// Verfall nach Leerlauf träte also nie ein. Eine Viertelstunde reicht
	// für Handy aus der Tasche, Passwort, Zwei-Faktor und Zustimmung.
	OAuthTTL = 15 * time.Minute

	// maxOAuthFlowsPerToken begrenzt die gleichzeitigen Vorgänge einer Box.
	//
	// Eine Box braucht genau einen; ein paar mehr fangen Wiederholungen nach
	// einem Neustart ab. Unbegrenzt könnte ein entwendetes Token den Speicher
	// des Relais füllen.
	maxOAuthFlowsPerToken = 4

	minOAuthStateLen   = 16
	maxOAuthStateLen   = 128
	maxAuthorizeURLLen = 8 << 10
	maxOAuthCodeLen    = 4 << 10

	// adobeLoginHostSuffix ist die einzige Gegenstelle, zu der das Relais
	// weiterleitet.
	//
	// Ohne diese Grenze wäre /oauth/adobe/start ein offener Umleiter: Wer an
	// ein Token kommt, könnte Links unter der Adresse des Relais verteilen,
	// die auf beliebige Seiten führen – ein Geschenk für Phishing.
	adobeLoginHostSuffix = ".adobelogin.com"
)

type oauthFlow struct {
	token        TokenID
	authorizeURL string
	code         string
	completed    bool
	createdAt    time.Time
}

// OAuthRegistry hält laufende Anmeldevorgänge ausschließlich im Speicher.
//
// Die Fotobox ist aus dem Internet nicht erreichbar und kann den Rücksprung
// des Fremddienstes deshalb nicht selbst empfangen. Das übernimmt das Relais:
// Es merkt sich, wohin das Handy geschickt werden soll, nimmt den Code beim
// Rücksprung entgegen und hält ihn bereit, bis die Box ihn abholt.
type OAuthRegistry struct {
	mu    sync.Mutex
	flows map[OAuthState]*oauthFlow
	now   func() time.Time
}

func NewOAuthRegistry() *OAuthRegistry {
	return &OAuthRegistry{flows: map[OAuthState]*oauthFlow{}, now: time.Now}
}

// Register meldet einen Anmeldevorgang der Box token an.
//
// Meldet dieselbe Box denselben Vorgang erneut an, gilt das als Wiederholung
// nach einem Übertragungsfehler und nicht als Konflikt.
func (r *OAuthRegistry) Register(token TokenID, state OAuthState, authorizeURL string) error {
	if err := ValidateOAuthState(state); err != nil {
		return err
	}

	if err := validateAuthorizeURL(authorizeURL); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	if flow, ok := r.flowLocked(state, now); ok {
		if flow.token != token {
			return fmt.Errorf("%w: state already in use", ErrOAuthInvalid)
		}

		if !flow.completed {
			flow.authorizeURL = authorizeURL
		}

		return nil
	}

	r.evictOldestLocked(token, now)
	r.flows[state] = &oauthFlow{token: token, authorizeURL: authorizeURL, createdAt: now}

	return nil
}

// AuthorizeURL liefert die Anmeldeseite, auf die das Handy weitergeleitet
// wird.
//
// Ein bereits abgeschlossener Vorgang liefert keine mehr: Eine zweite
// Anmeldung liefe ins Leere, weil der Code dafür schon vergeben ist.
func (r *OAuthRegistry) AuthorizeURL(state OAuthState) (string, error) {
	if ValidateOAuthState(state) != nil {
		return "", ErrOAuthUnknown
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	flow, ok := r.flowLocked(state, r.now())
	if !ok || flow.completed {
		return "", ErrOAuthUnknown
	}

	return flow.authorizeURL, nil
}

// Complete hinterlegt den Code aus dem Rücksprung des Fremddienstes.
//
// Ein Vorgang nimmt genau einen Code an. Lädt das Handy die Rücksprungseite
// neu, kommt derselbe Code noch einmal; das ist kein Fehler. Ein anderer Code
// für einen abgeschlossenen Vorgang wird dagegen abgewiesen – sonst könnte
// ein Dritter, der den state kennt, der Box sein eigenes Konto unterschieben.
func (r *OAuthRegistry) Complete(state OAuthState, code string) error {
	if ValidateOAuthState(state) != nil {
		return ErrOAuthUnknown
	}

	if err := validateOAuthCode(code); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	flow, ok := r.flowLocked(state, r.now())
	if !ok {
		return ErrOAuthUnknown
	}

	if flow.completed {
		if flow.code == code {
			return nil
		}

		return ErrOAuthUnknown
	}

	flow.code = code
	flow.completed = true

	return nil
}

// Collect holt den Code eines Vorgangs für die Box token ab.
//
// Solange der Rücksprung aussteht, ist der Code leer. Ein abgeholter Code
// verschwindet sofort: Er ist nur einmal einlösbar und hat danach im
// Speicher eines öffentlichen Dienstes nichts mehr verloren.
func (r *OAuthRegistry) Collect(token TokenID, state OAuthState) (string, error) {
	if err := ValidateOAuthState(state); err != nil {
		return "", err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	flow, ok := r.flowLocked(state, r.now())
	if !ok || flow.token != token {
		return "", ErrOAuthUnknown
	}

	if !flow.completed {
		return "", nil
	}

	delete(r.flows, state)

	return flow.code, nil
}

// PurgeExpired verwirft alle abgelaufenen Vorgänge.
//
// Abgelaufen ist ein Vorgang auch ohne diesen Aufruf – jeder Zugriff prüft
// das selbst. Hier geht es nur darum, dass vergessene Vorgänge nicht
// dauerhaft Speicher belegen.
func (r *OAuthRegistry) PurgeExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	for state, flow := range r.flows {
		if flow.expired(now) {
			delete(r.flows, state)
		}
	}
}

func (f *oauthFlow) expired(now time.Time) bool { return !now.Before(f.createdAt.Add(OAuthTTL)) }

// flowLocked liefert einen noch gültigen Vorgang. Der Aufrufer hält r.mu.
func (r *OAuthRegistry) flowLocked(state OAuthState, now time.Time) (*oauthFlow, bool) {
	flow, ok := r.flows[state]
	if !ok {
		return nil, false
	}

	if flow.expired(now) {
		delete(r.flows, state)
		return nil, false
	}

	return flow, true
}

// evictOldestLocked schafft Platz für einen weiteren Vorgang der Box token.
// Der Aufrufer hält r.mu.
func (r *OAuthRegistry) evictOldestLocked(token TokenID, now time.Time) {
	var (
		count  int
		oldest OAuthState
		at     time.Time
	)

	for state, flow := range r.flows {
		if flow.token != token {
			continue
		}

		if flow.expired(now) {
			delete(r.flows, state)
			continue
		}

		count++
		if oldest == "" || flow.createdAt.Before(at) {
			oldest, at = state, flow.createdAt
		}
	}

	if count >= maxOAuthFlowsPerToken {
		delete(r.flows, oldest)
	}
}

// ValidateOAuthState prüft die Form einer Vorgangskennung.
//
// Die Kennung landet in Adressen und im Speicher des Relais. Erlaubt ist
// deshalb nur, was in einer Adresse nicht maskiert werden muss, und nur so
// viel, wie eine Zufallskennung braucht.
func ValidateOAuthState(state OAuthState) error {
	if len(state) < minOAuthStateLen || len(state) > maxOAuthStateLen {
		return fmt.Errorf("%w: state must have %d to %d characters", ErrOAuthInvalid, minOAuthStateLen, maxOAuthStateLen)
	}

	for _, c := range []byte(state) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		default:
			return fmt.Errorf("%w: state contains invalid characters", ErrOAuthInvalid)
		}
	}

	return nil
}

func validateAuthorizeURL(raw string) error {
	if len(raw) > maxAuthorizeURLLen {
		return fmt.Errorf("%w: authorize url too long", ErrOAuthInvalid)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: authorize url: %v", ErrOAuthInvalid, err)
	}

	// Zugangsdaten in der Adresse sind ein bekannter Trick, einem Menschen
	// einen anderen Zielrechner vorzuspiegeln. Adobe braucht sie nie.
	if u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return fmt.Errorf("%w: authorize url must be a plain https url", ErrOAuthInvalid)
	}

	if port := u.Port(); port != "" && port != "443" {
		return fmt.Errorf("%w: authorize url must use the default https port", ErrOAuthInvalid)
	}

	host := strings.ToLower(u.Hostname())
	if !strings.HasSuffix(host, adobeLoginHostSuffix) || len(host) == len(adobeLoginHostSuffix) {
		return fmt.Errorf("%w: authorize url must point to %s", ErrOAuthInvalid, adobeLoginHostSuffix)
	}

	return nil
}

func validateOAuthCode(code string) error {
	if code == "" || len(code) > maxOAuthCodeLen {
		return fmt.Errorf("%w: code must have 1 to %d characters", ErrOAuthInvalid, maxOAuthCodeLen)
	}

	// Nur druckbares ASCII: Der Code wandert unverändert zur Box und ist
	// dort Teil einer Anfrage an Adobe. Steuerzeichen haben darin nichts
	// verloren.
	for _, c := range []byte(code) {
		if c < 0x21 || c > 0x7e {
			return fmt.Errorf("%w: code contains invalid characters", ErrOAuthInvalid)
		}
	}

	return nil
}
