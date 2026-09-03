package photo

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/pkg/data"
)

// ID identifiziert ein Foto.
//
// Die ID ist bewusst zeitlich sortierbar aufgebaut (Unix-Millis, links mit
// Nullen aufgefüllt, gefolgt von Zufall). Dadurch liefert die lexikographisch
// sortierte Iteration des Repositories automatisch die chronologische
// Reihenfolge, ohne dass ein Index nötig wäre.
type ID string

// NewID erzeugt eine neue, zeitlich sortierbare ID für den Zeitpunkt t.
func NewID(t time.Time) ID {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return ID(fmt.Sprintf("%013d-%s", t.UTC().UnixMilli(), hex.EncodeToString(buf[:])))
}

// Source beschreibt, woher ein Foto stammt.
type Source string

const (
	// SourceCamera markiert Fotos, die von der angeschlossenen Kamera
	// (PTP/MTP bzw. Tethering-Verzeichnis) eingespielt wurden.
	SourceCamera Source = "camera"

	// SourceUpload markiert Fotos, die ein Gast per Smartphone hochgeladen hat.
	SourceUpload Source = "upload"

	// SourceRelay markiert einen Internet-Upload über photoupld.
	SourceRelay Source = "relay"
)

func (s Source) String() string {
	switch s {
	case SourceCamera:
		return "Kamera"
	case SourceUpload:
		return "Gast-Upload"
	case SourceRelay:
		return "Internet-Upload"
	default:
		return string(s)
	}
}

// Photo ist das Aggregat eines einzelnen Bildes in der Fotobox.
type Photo struct {
	ID ID `json:"id,omitempty"`

	// Image verweist auf das SrcSet im Nago-Image-Subsystem. Unter demselben
	// Schlüssel liegt im Blob-Store zusätzlich das unveränderte Original,
	// welches für den Druck verwendet wird.
	Image image.ID `json:"img,omitempty"`

	// Name ist der ursprüngliche Dateiname, sofern bekannt.
	Name string `json:"name,omitempty"`

	// Source gibt an, ob das Foto von der Kamera oder von einem Gast stammt.
	Source Source `json:"src,omitempty"`

	// Width und Height sind die Abmessungen des Originals in Pixeln.
	Width  int `json:"w,omitempty"`
	Height int `json:"h,omitempty"`

	CreatedAt time.Time `json:"createdAt"`
}

func (p Photo) Identity() ID { return p.ID }

func (p Photo) WithIdentity(id ID) Photo {
	p.ID = id
	return p
}

func (p Photo) String() string {
	if p.Name != "" {
		return p.Name
	}

	return string(p.ID)
}

// Landscape meldet, ob das Foto im Querformat vorliegt.
func (p Photo) Landscape() bool { return p.Width >= p.Height }

// Repository speichert die Metadaten aller Fotos.
type Repository = data.Repository[Photo, ID]
