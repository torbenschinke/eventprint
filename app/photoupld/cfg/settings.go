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
	u.Path = strings.TrimSuffix(u.Path, "/") + "/upload"
	q := u.Query()
	q.Set("u", id)
	u.RawQuery = q.Encode()
	return u.String()
}
