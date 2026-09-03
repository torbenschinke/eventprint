// Package remote connects a private photobox to the public photoupld relay.
package remote

import (
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

type Job struct {
	ID       string              `json:"id"`
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
	req.Header.Set("Authorization", "Bearer "+c.token)
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
