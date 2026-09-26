package lightroom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// apiGet ruft einen Pfad der Lightroom-Schnittstelle ab.
//
// target ist entweder ein Pfad ab der Basisadresse ("/v2/catalog") oder eine
// bereits aufgelöste Blätteradresse. Lehnt Lightroom den Schlüssel mit 401 ab,
// obwohl er nach der Uhr noch gilt, wird er einmal erneuert und die Anfrage
// wiederholt: Adobe kann Schlüssel vorzeitig verwerfen, und der Besitzer soll
// davon nichts merken, solange der Refresh-Token noch taugt.
func (c *Client) apiGet(ctx context.Context, cfg Config, target string) (*http.Response, error) {
	if cfg.ClientID == "" {
		return nil, NotConfiguredError{Missing: []string{"Client-ID"}}
	}

	full := cfg.APIBaseURL + target
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		full = target
	}

	for attempt := 0; ; attempt++ {
		token, err := c.accessToken(ctx, cfg, attempt > 0)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return nil, fmt.Errorf("Lightroom-Adresse ist ungültig: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-API-Key", cfg.ClientID)

		res, err := cfg.HTTPClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("Lightroom ist nicht erreichbar: %w", err)
		}

		if res.StatusCode == http.StatusUnauthorized && attempt == 0 {
			_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
			_ = res.Body.Close()

			continue
		}

		return res, nil
	}
}

// getJSON ruft ein JSON-Dokument ab und dekodiert es in v.
func (c *Client) getJSON(ctx context.Context, cfg Config, target string, v any) error {
	res, err := c.apiGet(ctx, cfg, target)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return HTTPError{Service: "Lightroom", Status: res.StatusCode, Detail: errorDetail(stripJSONPrefixReader(res.Body))}
	}

	buf, err := io.ReadAll(io.LimitReader(res.Body, maxJSONResponse+1))
	if err != nil {
		return fmt.Errorf("Antwort von Lightroom bricht ab: %w", err)
	}

	if len(buf) > maxJSONResponse {
		return errors.New("Antwort von Lightroom ist unerwartet groß")
	}

	if err := json.Unmarshal(stripJSONPrefix(buf), v); err != nil {
		return fmt.Errorf("Antwort von Lightroom ist unverständlich: %w", err)
	}

	return nil
}

// stripJSONPrefix entfernt den Vorspann "while (1) {}", den Lightroom jeder
// JSON-Antwort voranstellt.
//
// Der Vorspann schützt Browser vor JSON-Hijacking und macht die Antwort für
// einen gewöhnlichen JSON-Decoder unlesbar. Gesucht wird nicht die exakte
// Zeichenfolge, sondern alles bis zur ersten schließenden Klammer: Die
// Schreibweise mit oder ohne Leerzeichen ist nirgends zugesichert.
func stripJSONPrefix(buf []byte) []byte {
	buf = bytes.TrimSpace(buf)
	if !bytes.HasPrefix(buf, []byte("while")) {
		return buf
	}

	i := bytes.IndexByte(buf, '}')
	if i < 0 {
		return buf
	}

	return bytes.TrimSpace(buf[i+1:])
}

func stripJSONPrefixReader(r io.Reader) io.Reader {
	buf, _ := io.ReadAll(io.LimitReader(r, 4096))

	return bytes.NewReader(stripJSONPrefix(buf))
}

// links ist der Blätterteil einer Lightroom-Liste.
//
// base ist die Adresse, gegen die Lightroom die relativen Verweise auflöst;
// sie steht im Dokument selbst, weil die Verweise oft nicht relativ zur
// abgerufenen Adresse sind.
type listEnvelope struct {
	Base  string `json:"base"`
	Links struct {
		Next *struct {
			Href string `json:"href"`
		} `json:"next"`
	} `json:"links"`
}

// nextCursor löst den Verweis auf die nächste Seite zu einem Pfad ab der
// Basisadresse auf.
//
// Der Verweis kann absolut ("/v2/catalogs/…"), relativ zur mitgelieferten
// base oder relativ zur abgerufenen Adresse sein. Das Ergebnis wird auf den
// Rechner der Lightroom-Schnittstelle festgenagelt, denn es wandert als
// Cursor durch die Oberfläche zurück, und ein Verweis auf einen fremden
// Rechner bekäme den Zugriffsschlüssel des Besitzers mitgeschickt.
func nextCursor(cfg Config, requested string, env listEnvelope) (string, error) {
	if env.Links.Next == nil || strings.TrimSpace(env.Links.Next.Href) == "" {
		return "", nil
	}

	api, err := url.Parse(cfg.APIBaseURL)
	if err != nil {
		return "", fmt.Errorf("Lightroom-Adresse ist ungültig: %w", err)
	}

	ref, err := url.Parse(strings.TrimSpace(env.Links.Next.Href))
	if err != nil {
		return "", fmt.Errorf("Lightroom liefert einen ungültigen Verweis: %w", err)
	}

	from, err := url.Parse(requested)
	if err != nil {
		return "", err
	}

	if env.Base != "" {
		if base, err := url.Parse(env.Base); err == nil {
			from = from.ResolveReference(base)
		}
	}

	next := from.ResolveReference(ref)
	if next.Scheme != api.Scheme || next.Host != api.Host {
		return "", errors.New("Lightroom verweist auf einen fremden Rechner")
	}

	return next.RequestURI(), nil
}

// cursorURL wandelt einen Cursor der Oberfläche zurück in eine Adresse.
func cursorURL(cfg Config, cursor string) (string, error) {
	if !strings.HasPrefix(cursor, "/") || strings.HasPrefix(cursor, "//") {
		return "", errors.New("ungültige Seitenmarke")
	}

	api, err := url.Parse(cfg.APIBaseURL)
	if err != nil {
		return "", fmt.Errorf("Lightroom-Adresse ist ungültig: %w", err)
	}

	return api.Scheme + "://" + api.Host + cursor, nil
}

// catalog liefert die Kennung des Katalogs.
//
// Jeder Lightroom-Nutzer hat genau einen Katalog. Die Kennung ändert sich nur
// mit einem anderen Konto, deshalb wird sie bis zur nächsten Anmeldung im
// Speicher gehalten.
func (c *Client) catalog(ctx context.Context, cfg Config) (string, error) {
	c.cacheMu.Lock()
	id := c.catalogID
	c.cacheMu.Unlock()

	if id != "" {
		return id, nil
	}

	var out struct {
		ID string `json:"id"`
	}

	if err := c.getJSON(ctx, cfg, "/v2/catalog", &out); err != nil {
		return "", err
	}

	if out.ID == "" {
		return "", errors.New("Lightroom liefert keinen Katalog – ist das Konto für Lightroom freigeschaltet?")
	}

	c.cacheMu.Lock()
	c.catalogID = out.ID
	c.cacheMu.Unlock()

	return out.ID, nil
}

// account liefert den Anzeigenamen des Kontos.
func (c *Client) account(ctx context.Context, cfg Config) (string, error) {
	c.cacheMu.Lock()
	name := c.accountName
	c.cacheMu.Unlock()

	if name != "" {
		return name, nil
	}

	var out struct {
		FullName  string `json:"full_name"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
	}

	if err := c.getJSON(ctx, cfg, "/v2/account", &out); err != nil {
		return "", err
	}

	name = strings.TrimSpace(out.FullName)
	if name == "" {
		name = strings.TrimSpace(out.FirstName + " " + out.LastName)
	}

	if name == "" {
		name = strings.TrimSpace(out.Email)
	}

	c.cacheMu.Lock()
	c.accountName = name
	c.cacheMu.Unlock()

	return name, nil
}

// maxRendition begrenzt ein heruntergeladenes Bild. Ein 2048er-JPEG hat
// wenige Megabyte; die Grenze fängt nur eine Gegenstelle ab, die etwas ganz
// anderes schickt, bevor sie den Speicher des Pi füllt.
const maxRendition = 60 << 20

// openRendition öffnet eine Bildstufe eines Fotos.
func (c *Client) openRendition(ctx context.Context, cfg Config, assetID, kind string) (io.ReadCloser, error) {
	if strings.TrimSpace(assetID) == "" {
		return nil, errors.New("kein Foto angegeben")
	}

	cid, err := c.catalog(ctx, cfg)
	if err != nil {
		return nil, err
	}

	res, err := c.apiGet(ctx, cfg, "/v2/catalogs/"+url.PathEscape(cid)+"/assets/"+url.PathEscape(assetID)+"/renditions/"+url.PathEscape(kind))
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		defer res.Body.Close()
		return nil, HTTPError{Service: "Lightroom", Status: res.StatusCode, Detail: errorDetail(stripJSONPrefixReader(res.Body))}
	}

	return limitedReadCloser{Reader: io.LimitReader(res.Body, maxRendition), Closer: res.Body}, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func isStatus(err error, status int) bool {
	var httpErr HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status
}
