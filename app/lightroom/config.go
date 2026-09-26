// Package lightroom holt Fotos aus der Lightroom-Cloud des Besitzers auf die
// Fotobox.
//
// Die Fotobox hat keine öffentliche Adresse und nur einen Touchscreen. Die
// Anmeldung bei Adobe findet deshalb auf dem Telefon des Besitzers statt: Die
// Box zeigt einen QR-Code, das Telefon meldet sich an, und Adobe schickt den
// Autorisierungscode an den öffentlichen Upload-Dienst, bei dem die Box ihn
// abholt. Dieses Paket ist die Seite der Box – der OAuth-Client, die
// Verwaltung der Schlüssel und der Zugriff auf die Lightroom-Schnittstelle.
package lightroom

import (
	"net/http"
	"strings"
	"time"
)

// Voreinstellungen für die Adobe-Endpunkte.
//
// offline_access ist der Grund, warum die Box nach einer einzigen Anmeldung
// wochenlang ohne Telefon auskommt: Ohne diesen Scope gibt es keinen
// Refresh-Token, und nach einem Tag müsste jemand erneut den QR-Code scannen.
const (
	DefaultScopes     = "openid,AdobeID,lr_partner_apis,lr_partner_rendition_apis,offline_access"
	DefaultIMSBaseURL = "https://ims-na1.adobelogin.com"
	DefaultAPIBaseURL = "https://lr.adobe.io"
)

// defaultHTTPTimeout begrenzt eine einzelne Anfrage, wenn der Aufrufer keinen
// eigenen Client mitgibt. Ein hängendes Funknetz soll die Oberfläche nicht
// unbegrenzt warten lassen; ein 2048er-JPEG ist selbst über schwaches WLAN in
// dieser Zeit angekommen.
const defaultHTTPTimeout = 60 * time.Second

// Config beschreibt die Anbindung an Adobe und an den Relay.
type Config struct {
	// ClientID und ClientSecret stammen aus der Adobe Developer Console.
	// Die ClientID dient zugleich als X-API-Key der Lightroom-Schnittstelle.
	ClientID     string
	ClientSecret string

	// RelayURL ist die öffentliche Adresse des Upload-Dienstes, z. B.
	// https://upload.example.de. Unter ihr empfängt der Relay die Weiterleitung
	// von Adobe, die die Box selbst nicht empfangen kann.
	RelayURL string

	// RelayToken weist die Box beim Relay aus.
	RelayToken string

	// Scopes, IMSBaseURL und APIBaseURL sind nur für Tests oder eine andere
	// Adobe-Region zu ändern; leer gelten die Voreinstellungen.
	Scopes     string
	IMSBaseURL string
	APIBaseURL string

	// HTTPClient ist optional; ohne ihn gilt ein Client mit Zeitlimit.
	HTTPClient *http.Client
}

// normalized füllt Voreinstellungen ein und entfernt Schrägstriche am Ende.
//
// Die Adressen werden von Hand eingetragen. Ein abschließender Schrägstrich
// ist dort so üblich wie unsichtbar und würde sonst zu "//" in jedem Pfad.
func (c Config) normalized() Config {
	c.ClientID = strings.TrimSpace(c.ClientID)
	c.ClientSecret = strings.TrimSpace(c.ClientSecret)
	c.RelayToken = strings.TrimSpace(c.RelayToken)
	c.RelayURL = strings.TrimRight(strings.TrimSpace(c.RelayURL), "/")
	c.IMSBaseURL = strings.TrimRight(strings.TrimSpace(c.IMSBaseURL), "/")
	c.APIBaseURL = strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
	c.Scopes = strings.TrimSpace(c.Scopes)

	if c.Scopes == "" {
		c.Scopes = DefaultScopes
	}

	if c.IMSBaseURL == "" {
		c.IMSBaseURL = DefaultIMSBaseURL
	}

	if c.APIBaseURL == "" {
		c.APIBaseURL = DefaultAPIBaseURL
	}

	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	return c
}

// requireOAuth prüft, ob alles für Anmeldung und Token-Tausch eingetragen ist.
//
// Die Prüfung steht vor dem ersten Netzzugriff: Ein QR-Code, der erst nach dem
// Scannen an einem fehlenden Secret scheitert, kostet den Besitzer eine
// vollständige Anmeldung auf dem Telefon.
func (c Config) requireOAuth() error {
	var missing []string

	if c.ClientID == "" {
		missing = append(missing, "Client-ID")
	}

	if c.ClientSecret == "" {
		missing = append(missing, "Client-Secret")
	}

	if c.RelayURL == "" {
		missing = append(missing, "Relay-Adresse")
	}

	if c.RelayToken == "" {
		missing = append(missing, "Relay-Token")
	}

	if len(missing) > 0 {
		return NotConfiguredError{Missing: missing}
	}

	return nil
}

// relayRedirectURI ist die Adresse, an die Adobe nach der Anmeldung
// weiterleitet. Sie muss in der Adobe Developer Console als erlaubte
// Weiterleitung eingetragen sein.
func (c Config) relayRedirectURI() string {
	return c.RelayURL + "/oauth/adobe/callback"
}
