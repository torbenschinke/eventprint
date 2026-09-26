package lightroom

import (
	"io"
	"strings"
	"sync"
	"time"
)

// Client hält, was sich die Anwendungsfälle teilen müssen: die Einstellungen,
// die Schlüsselablage und die Sperre um die Schlüsselerneuerung.
//
// Die Anwendungsfälle bekommen ihn über den Konstruktor und nicht über eine
// Paketvariable. Sonst hinge ihr Verhalten an einer Initialisierung, die an
// der Aufrufstelle unsichtbar ist, und zwei Boxen in einem Test sähen sich
// gegenseitig.
type Client struct {
	config func() Config
	store  TokenStore
	now    func() time.Time

	// tokenMu umschließt Laden, Erneuern und Löschen der Schlüssel. Die
	// Übersicht lädt Dutzende Vorschaubilder gleichzeitig; liefe für jedes
	// eine eigene Erneuerung, würde Adobe den Refresh-Token nach der ersten
	// womöglich schon verworfen haben, und alle übrigen scheiterten.
	tokenMu sync.Mutex

	// cacheMu schützt die Zwischenspeicher. Katalog und Name ändern sich nur
	// mit einer neuen Anmeldung; sie bei jedem Aufruf neu zu holen, verdoppelte
	// die Wartezeit jedes Bildschirms.
	cacheMu     sync.Mutex
	catalogID   string
	accountName string
}

// NewClient bindet den Zugriff an einen Einstellungslader und eine Ablage.
//
// Die Einstellungen kommen als Funktion, damit eine geänderte Client-ID oder
// Relay-Adresse ohne Neustart gilt.
func NewClient(config func() Config, store TokenStore) *Client {
	return &Client{config: config, store: store, now: time.Now}
}

func (c *Client) cfg() Config {
	return c.config().normalized()
}

// forget verwirft alles, was an der bisherigen Anmeldung hängt.
func (c *Client) forget() {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.catalogID = ""
	c.accountName = ""
}

// maxErrorDetail begrenzt den Auszug einer Fehlerantwort. Er landet in einer
// Meldung auf dem Touchscreen, und eine HTML-Fehlerseite hat dort nichts
// verloren.
const maxErrorDetail = 200

// errorDetail liest einen kurzen Auszug einer Fehlerantwort.
func errorDetail(r io.Reader) string {
	buf, _ := io.ReadAll(io.LimitReader(r, 4096))
	s := strings.TrimSpace(string(buf))

	if strings.HasPrefix(s, "<") {
		return ""
	}

	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxErrorDetail {
		s = string(r[:maxErrorDetail]) + "…"
	}

	return s
}
