package lightroom

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// relayStateLifetime ist, wie lange der Relay einen angemeldeten Vorgang
// aufbewahrt. Die Box nennt der Oberfläche diese Frist, damit sie einen
// abgelaufenen QR-Code von sich aus ersetzt, statt den Besitzer ins Leere
// scannen zu lassen.
const relayStateLifetime = 15 * time.Minute

// refreshMargin ist der Vorlauf, mit dem ein Zugriffsschlüssel vor seinem
// Ablauf erneuert wird. Eine Anfrage, die mit einem noch gültigen Schlüssel
// abgeschickt wird, kann bei Adobe schon mit einem abgelaufenen ankommen.
const refreshMargin = 60 * time.Second

// maxJSONResponse begrenzt JSON-Antworten. Eine Seite Lightroom-Metadaten ist
// wenige hundert Kilobyte groß; alles darüber ist ein Fehler der Gegenstelle
// und soll nicht den Speicher des Pi füllen.
const maxJSONResponse = 16 << 20

// newState erzeugt den Zufallswert, der Anmeldung und Abholung verknüpft.
//
// crypto/rand, weil der Wert beim Relay als einziger Schlüssel zum
// Autorisierungscode dient: Wer ihn errät, bekommt Zugang zu den Fotos.
func newState() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("Zufallswert für die Anmeldung kann nicht erzeugt werden: %w", err)
	}

	return hex.EncodeToString(b[:]), nil
}

// validState prüft einen State, bevor er in eine Anfrage an den Relay geht.
func validState(s string) bool {
	if len(s) != 32 {
		return false
	}

	_, err := hex.DecodeString(s)

	return err == nil
}

// authorizeURL ist die Anmeldeseite von Adobe, die das Telefon öffnet.
func authorizeURL(cfg Config, state string) string {
	q := url.Values{}
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", cfg.relayRedirectURI())
	q.Set("scope", cfg.Scopes)
	q.Set("response_type", "code")
	q.Set("state", state)

	return cfg.IMSBaseURL + "/ims/authorize/v2?" + q.Encode()
}

// registerAtRelay hinterlegt die Anmeldeseite beim Relay und liefert die
// kurze Adresse für den QR-Code.
//
// Der Umweg über den Relay hat zwei Gründe: Die Adobe-Adresse ist mit allen
// Parametern so lang, dass der QR-Code auf dem kleinen Display nicht mehr
// zuverlässig zu scannen ist, und der Relay muss den State ohnehin kennen,
// um den zurückkommenden Code zuordnen zu können.
func registerAtRelay(ctx context.Context, cfg Config, state, authURL string) (string, error) {
	body, err := json.Marshal(struct {
		State        string `json:"state"`
		AuthorizeURL string `json:"authorizeUrl"`
	}{state, authURL})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.RelayURL+"/api/v1/oauth/adobe", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("Relay-Adresse ist ungültig: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.RelayToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("Relay ist nicht erreichbar: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		return "", HTTPError{Service: "Relay", Status: res.StatusCode, Detail: errorDetail(res.Body)}
	}

	var out struct {
		StartURL string `json:"startUrl"`
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxJSONResponse)).Decode(&out); err != nil {
		return "", fmt.Errorf("Antwort des Relays ist unverständlich: %w", err)
	}

	u, err := url.Parse(out.StartURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", errors.New("Relay liefert keine gültige Startadresse für den QR-Code")
	}

	return out.StartURL, nil
}

// pollRelay fragt einmal, ob das Telefon die Anmeldung abgeschlossen hat.
//
// found ist false, wenn der Relay den State nicht (mehr) kennt – dann ist der
// QR-Code verfallen, und nur ein neuer hilft.
func pollRelay(ctx context.Context, cfg Config, state string) (code string, found bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		cfg.RelayURL+"/api/v1/oauth/adobe?state="+url.QueryEscape(state), nil)
	if err != nil {
		return "", false, fmt.Errorf("Relay-Adresse ist ungültig: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+cfg.RelayToken)
	req.Header.Set("Accept", "application/json")

	res, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("Relay ist nicht erreichbar: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return "", false, nil
	}

	if res.StatusCode != http.StatusOK {
		return "", false, HTTPError{Service: "Relay", Status: res.StatusCode, Detail: errorDetail(res.Body)}
	}

	var out struct {
		Code  string `json:"code"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxJSONResponse)).Decode(&out); err != nil {
		return "", false, fmt.Errorf("Antwort des Relays ist unverständlich: %w", err)
	}

	// Das Protokoll kennt nur den Code. Reicht der Relay dennoch einen Fehler
	// von Adobe durch (etwa eine abgelehnte Freigabe), soll die Box nicht
	// weiter auf einen Code warten, der nie kommt.
	if out.Error != "" {
		return "", true, fmt.Errorf("Adobe hat die Anmeldung abgelehnt: %s", out.Error)
	}

	return out.Code, true, nil
}

// exchangeTokens ruft den Token-Endpunkt von Adobe auf.
//
// Für den Autorisierungscode wie für die Erneuerung ist es derselbe Endpunkt;
// nur die Formularfelder unterscheiden sich.
func exchangeTokens(ctx context.Context, cfg Config, now time.Time, form url.Values) (Tokens, error) {
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.IMSBaseURL+"/ims/token/v3", strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, fmt.Errorf("Adresse der Adobe-Anmeldung ist ungültig: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	res, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return Tokens{}, fmt.Errorf("Adobe-Anmeldung ist nicht erreichbar: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return Tokens{}, HTTPError{Service: "Adobe-Anmeldung", Status: res.StatusCode, Detail: errorDetail(res.Body)}
	}

	var out struct {
		AccessToken  string  `json:"access_token"`
		RefreshToken string  `json:"refresh_token"`
		ExpiresIn    float64 `json:"expires_in"`
	}

	if err := json.NewDecoder(io.LimitReader(res.Body, maxJSONResponse)).Decode(&out); err != nil {
		return Tokens{}, fmt.Errorf("Antwort der Adobe-Anmeldung ist unverständlich: %w", err)
	}

	if out.AccessToken == "" {
		return Tokens{}, errors.New("Adobe-Anmeldung liefert keinen Zugriffsschlüssel")
	}

	// Fehlt die Laufzeit, gilt eine Stunde. Zu kurz geschätzt kostet das nur
	// eine frühe Erneuerung; zu lang geschätzt schlüge jede Anfrage fehl, bis
	// Lightroom den Schlüssel mit 401 ablehnt.
	lifetime := time.Duration(out.ExpiresIn * float64(time.Second))
	if lifetime <= 0 {
		lifetime = time.Hour
	}

	return Tokens{
		AccessToken:  out.AccessToken,
		RefreshToken: out.RefreshToken,
		ExpiresAt:    now.Add(lifetime),
	}, nil
}

// accessToken liefert einen gültigen Zugriffsschlüssel und erneuert ihn, wenn
// er demnächst abläuft oder force gesetzt ist.
//
// Lehnt Adobe die Erneuerung mit 400 oder 401 ab, ist der Refresh-Token
// widerrufen oder verfallen. Die Schlüssel werden dann gelöscht: Sie
// aufzubewahren hieße, bei jedem Bildschirm erneut zu scheitern, statt den
// Besitzer einmal klar zur neuen Anmeldung zu schicken.
func (c *Client) accessToken(ctx context.Context, cfg Config, force bool) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	t, ok, err := c.store.Load()
	if err != nil {
		return "", err
	}

	if !ok {
		return "", NotConnectedError{}
	}

	if !force && t.AccessToken != "" && t.ExpiresAt.Sub(c.now()) > refreshMargin {
		return t.AccessToken, nil
	}

	if t.RefreshToken == "" {
		c.dropTokensLocked()
		return "", NotConnectedError{Reason: "Anmeldung abgelaufen"}
	}

	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return "", NotConfiguredError{Missing: []string{"Client-ID oder Client-Secret"}}
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", t.RefreshToken)

	fresh, err := exchangeTokens(ctx, cfg, c.now(), form)
	if err != nil {
		var httpErr HTTPError
		if errors.As(err, &httpErr) && (httpErr.Status == http.StatusBadRequest || httpErr.Status == http.StatusUnauthorized) {
			c.dropTokensLocked()
			return "", NotConnectedError{Reason: "Adobe hat die Anmeldung widerrufen"}
		}

		return "", err
	}

	// Adobe liefert bei der Erneuerung nicht zwingend einen neuen
	// Refresh-Token. Der alte gilt dann weiter und darf nicht verloren gehen.
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = t.RefreshToken
	}

	if err := c.store.Save(fresh); err != nil {
		return "", err
	}

	return fresh.AccessToken, nil
}

// dropTokensLocked löscht die Schlüssel; der Aufrufer hält tokenMu.
//
// Ein Fehler beim Löschen wird verschluckt: Die Meldung an den Besitzer ist
// ohnehin, dass er sich neu verbinden muss, und die neue Anmeldung
// überschreibt die Datei.
func (c *Client) dropTokensLocked() {
	_ = c.store.Delete()
	c.forget()
}
