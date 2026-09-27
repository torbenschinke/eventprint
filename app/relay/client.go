// Package relay verbindet das Gerät mit dem öffentlichen Upload-Dienst.
//
// Das Gerät selbst bleibt von außen unerreichbar. Es fragt den Dienst
// regelmäßig nach neuen Bildern, holt sie ab und bestätigt die Übernahme.
package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/torbenschinke/eventprint/app/printing"
)

type Options struct {
	URL      string
	Token    string
	Interval time.Duration
}

func (o Options) Enabled() bool {
	return o.Configured() && strings.TrimSpace(o.Token) != ""
}

// Configured meldet, dass ein Upload-Dienst eingetragen ist.
//
// Getrennt von [Options.Enabled], weil der Unterschied wesentlich ist: Wer nur
// die Adresse einträgt und das Token vergisst, hat etwas gewollt, das nicht
// geschieht. Ohne diese Unterscheidung fiele der Fall mit "gar nicht
// eingerichtet" zusammen und bliebe stumm.
func (o Options) Configured() bool {
	return strings.TrimSpace(o.URL) != ""
}

// Job ist ein wartender Upload beim Dienst.
type Job struct {
	ID string `json:"id"`

	// Template ist das Layout, das ein Gast gewählt hat; leer bei Uploads
	// in den Eingang.
	Template printing.TemplateID `json:"template"`
	Filename string              `json:"filename"`
}

type Client struct {
	base  *url.URL
	token string
	http  *http.Client
}

type HTTPError struct {
	StatusCode int
	Status     string

	// Body ist die Begründung des Servers, gekürzt.
	//
	// Sie wurde vorher weggeworfen. Übrig blieb "photoupld returned 400 Bad
	// Request" – eine Meldung, aus der niemand ableiten kann, ob das Token
	// unbekannt ist, abgelaufen oder nur ohne die nötige Rolle.
	Body string
}

func (e HTTPError) Error() string {
	msg := "photoupld returned " + e.Status
	if e.Body != "" {
		msg += ": " + e.Body
	}

	// Bei genau diesem Fall liegt die Ursache fast immer am Token, und der
	// Server kann es nicht sagen: Nago reicht ein unbekanntes Token als
	// anonymen Nutzer durch, und die fehlende Berechtigung wird erst später
	// zum Fehler. "Unbekannt" und "ohne Rolle" sehen von außen gleich aus.
	if e.StatusCode == http.StatusBadRequest {
		msg += " (Token prüfen: existiert es in photoupld, ist es gültig, und trägt es die Rolle Fotobox-Relay?)"
	}

	return msg
}

func NewClient(opts Options) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(opts.URL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid photoupld URL %q", opts.URL)
	}
	return &Client{base: base, token: opts.Token, http: &http.Client{Timeout: 60 * time.Second}}, nil
}

func (c *Client) OpenSession(ctx context.Context) (string, error) {
	var out struct {
		UploadURL string `json:"uploadUrl"`
	}
	if err := c.json(ctx, http.MethodPost, "/api/v1/session", nil, &out); err != nil {
		return "", err
	}
	return out.UploadURL, nil
}

func (c *Client) Jobs(ctx context.Context) ([]Job, error) {
	var out []Job
	err := c.json(ctx, http.MethodGet, "/api/v1/jobs", nil, &out)
	return out, err
}

func (c *Client) Image(ctx context.Context, id string) (io.ReadCloser, string, error) {
	resp, err := c.do(ctx, http.MethodGet, "/api/v1/job/image", url.Values{"id": {id}})
	if err != nil {
		return nil, "", err
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

func (c *Client) Ack(ctx context.Context, id string) error {
	var out struct {
		Acknowledged bool `json:"acknowledged"`
	}
	return c.json(ctx, http.MethodDelete, "/api/v1/job", url.Values{"id": {id}}, &out)
}

// PairingOutcome ist die Antwort des Dienstes auf einen Code.
type PairingOutcome string

const (
	PairingPaired  PairingOutcome = "paired"
	PairingInvalid PairingOutcome = "invalid"
	PairingExpired PairingOutcome = "expired"
	PairingLocked  PairingOutcome = "locked"
)

// RequestPairing bittet den Dienst um einen Code an mail. Es braucht kein
// Token; die Antwort ist die Kennung der Kopplung.
func (c *Client) RequestPairing(ctx context.Context, mail, device string) (string, error) {
	var out struct {
		Pairing string `json:"pairing"`
	}

	err := c.post(ctx, "/api/v1/pairing", map[string]string{"mail": mail, "device": device}, &out)
	if err == nil && out.Pairing == "" {
		err = fmt.Errorf("photoupld returned no pairing id")
	}

	return out.Pairing, err
}

// ConfirmPairing schickt den Code und bekommt bei Erfolg das Token.
func (c *Client) ConfirmPairing(ctx context.Context, pairing, code string) (PairingOutcome, string, error) {
	var out struct {
		Status PairingOutcome `json:"status"`
		Token  string         `json:"token"`
	}

	if err := c.post(ctx, "/api/v1/pairing/confirm", map[string]string{"pairing": pairing, "code": code}, &out); err != nil {
		return "", "", err
	}

	return out.Status, out.Token, nil
}

// post schickt JSON ohne Token.
func (c *Client) post(ctx context.Context, path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}

	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("photoupld request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return HTTPError{StatusCode: resp.StatusCode, Status: resp.Status, Body: strings.TrimSpace(string(msg))}
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(out); err != nil {
		return fmt.Errorf("cannot decode photoupld response: %w", err)
	}

	return nil
}

func (c *Client) json(ctx context.Context, method, path string, query url.Values, out any) error {
	resp, err := c.do(ctx, method, path, query)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("cannot decode photoupld response: %w", err)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values) (*http.Response, error) {
	u := *c.base
	u.Path = strings.TrimRight(c.base.Path, "/") + path
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("photoupld request failed: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()

		// Begrenzt lesen: Ein fremder Dienst darf nicht bestimmen, wie viel
		// Speicher eine Fehlermeldung belegt.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))

		return nil, HTTPError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(body)),
		}
	}
	return resp, nil
}
