package cfgphotobox

import (
	"net/url"
	"strings"

	"github.com/worldiety/enum"
	"go.wdy.de/nago/application/settings"
)

// Settings sind die global gültigen Einstellungen der Fotobox.
//
// Sie werden über das Admin-Center gepflegt, damit die Fotobox vor Ort ohne
// Umgebungsvariablen und ohne Neustart eingerichtet werden kann.
type Settings struct {
	_ any `title:"Fotobox" description:"Titel der Veranstaltung und die Adresse, unter der Gäste die Fotobox erreichen."`

	// EventTitle erscheint als Überschrift auf dem Startbildschirm.
	EventTitle string `json:"eventTitle,omitempty" label:"Titel der Veranstaltung" supportingText:"Erscheint groß auf dem Fotobox-Display, z. B. Hochzeit von Anna & Ben."`

	// PublicURL ist die von außen erreichbare Adresse der Fotobox.
	//
	// Sie muss von Hand setzbar sein, weil Nago den öffentlichen Namen sonst
	// aus dem Host-Header der ersten Verbindung ableitet. Hinter einem Reverse
	// Proxy stimmt der nicht: Dort landet oft die interne Adresse oder das
	// falsche Schema im QR-Code, und kein Gast kommt auf die Upload-Seite.
	PublicURL string `json:"publicUrl,omitempty" label:"Öffentliche Adresse" supportingText:"Vollständig inklusive Schema, z. B. https://fotobox.example.de. Leer: automatisch aus der Verbindung ermitteln."`
}

func (Settings) GlobalSettings() bool { return true }

// Registrierung im offenen Summentyp der globalen Einstellungen. Ohne diese
// Zeile taucht die Seite im Admin-Center nicht auf.
//
// Zum verpflichtenden Namen siehe [printing.Settings]: Ohne ihn kollidiert
// der Diskriminator mit jedem anderen Typ namens "Settings".
var _ = enum.Variant[settings.GlobalSettings, Settings](
	enum.Rename[Settings]("eventprint.booth.settings"),
)

// PublicURLFor bildet die vollständige Adresse eines Pfades.
//
// Ist keine öffentliche Adresse eingestellt, greift fallback – also die von
// Nago aus der Verbindung abgeleitete Adresse.
func (s Settings) PublicURLFor(path string, fallback func() string) string {
	base := strings.TrimSpace(s.PublicURL)
	if base == "" {
		return fallback()
	}

	// Ein vergessenes Schema würde als relativer Pfad interpretiert und der
	// QR-Code wäre unbrauchbar.
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}

	u, err := url.Parse(base)
	if err != nil {
		return fallback()
	}

	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + strings.TrimPrefix(path, "/")

	return u.String()
}
