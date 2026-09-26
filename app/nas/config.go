package nas

import (
	"net"
	"strings"
)

// Config sind die Zugangsdaten eines NAS.
//
// Das Kennwort steht im Klartext in den Einstellungen. Anders geht es nicht:
// SMB verlangt es bei jeder Anmeldung, und die Box startet ohne jemanden, der
// ein Hauptkennwort eintippen könnte. Die Einstellungen liegen deshalb nur für
// den Dienstnutzer lesbar auf der Karte. Wer das nicht möchte, legt auf dem
// NAS einen eigenen Benutzer an, der nur die Fotos lesen darf.
type Config struct {
	// Host ist Name oder Adresse des NAS, etwa "diskstation.local" oder
	// "192.168.178.20". Ein Port nach Doppelpunkt ist erlaubt.
	Host string `json:"host,omitempty"`

	// User darf eine Domäne tragen: "HEIM\\anna".
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`

	// Share ist die Freigabe, bei Synology meist "photo" oder "home".
	Share string `json:"share,omitempty"`
}

// Configured meldet, ob genug eingetragen ist, um Fotos zu suchen.
func (c Config) Configured() bool {
	return c.Host != "" && c.User != "" && c.Share != ""
}

// Normalized räumt eine Eingabe auf. Wer die Adresse aus dem Finder oder dem
// Windows-Explorer kopiert, bringt "smb://diskstation/photo" oder
// "\\diskstation\photo" mit; beides meint Host und Freigabe.
func (c Config) Normalized() Config {
	c.Host = strings.TrimSpace(c.Host)
	c.User = strings.TrimSpace(c.User)
	c.Share = strings.Trim(strings.TrimSpace(c.Share), `/\`)

	host := strings.TrimPrefix(c.Host, "smb://")
	host = strings.TrimLeft(strings.ReplaceAll(host, `\`, "/"), "/")
	if h, share, ok := strings.Cut(host, "/"); ok {
		host = h
		if c.Share == "" {
			c.Share = strings.Trim(share, "/")
		}
	}

	c.Host = host

	return c
}

// Title beschreibt die Freigabe für die Oberfläche.
func (c Config) Title() string {
	if c.Share == "" {
		return c.Host
	}

	return c.Share + " auf " + c.Host
}

// address ist das Ziel der TCP-Verbindung. SMB spricht seit Windows 2000
// direkt auf Port 445; NetBIOS auf 139 braucht kein heutiges NAS mehr.
func (c Config) address() string {
	if _, _, err := net.SplitHostPort(c.Host); err == nil {
		return c.Host
	}

	return net.JoinHostPort(strings.Trim(c.Host, "[]"), "445")
}

// credentials trennt die Domäne vom Benutzernamen.
func (c Config) credentials() (domain, user string) {
	if d, u, ok := strings.Cut(c.User, `\`); ok {
		return d, u
	}

	return "", c.User
}
