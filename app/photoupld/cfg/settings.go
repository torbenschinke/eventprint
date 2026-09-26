package photoupld

import (
	"net/url"
	"strings"

	"github.com/worldiety/enum"
	"go.wdy.de/nago/application/settings"
)

type Settings struct {
	_         any    `title:"Foto-Upload" description:"Öffentliche Adresse des Upload-Dienstes."`
	PublicURL string `json:"publicUrl,omitempty" label:"Öffentliche Adresse" supportingText:"Zum Beispiel https://upload.example.de. Leer: aus der aktuellen Verbindung ermitteln."`
}

func (Settings) GlobalSettings() bool { return true }

var _ = enum.Variant[settings.GlobalSettings, Settings](enum.Rename[Settings]("eventprint.upld.settings"))

func (s Settings) UploadURL(id string, fallback func() string) string {
	return s.publicURL("/upload", url.Values{"u": {id}}, fallback)
}

// publicURL bildet eine von außen erreichbare Adresse des Dienstes.
//
// Alle Adressen hängen an derselben Basis. Liefe eine davon an der
// eingestellten öffentlichen Adresse vorbei, zeigte die Box einen QR-Code,
// der nur im lokalen Netz funktioniert.
func (s Settings) publicURL(path string, query url.Values, fallback func() string) string {
	base := strings.TrimSpace(s.PublicURL)
	if base == "" {
		base = fallback()
	}
	u, err := url.Parse(base)
	if err != nil {
		return ""
	}
	if !strings.Contains(base, "://") {
		u, _ = url.Parse("https://" + base)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + path
	u.RawQuery = query.Encode()
	return u.String()
}
