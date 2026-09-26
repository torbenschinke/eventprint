package lightroom

import (
	"encoding/json"
	"path"
	"strconv"
	"strings"
	"time"
)

// Authorization ist ein begonnener Verbindungsaufbau.
type Authorization struct {
	// State verknüpft die Anmeldung auf dem Telefon mit der Abholung durch
	// die Box; die Oberfläche reicht ihn an [AwaitConnect] weiter.
	State string

	// StartURL ist die kurze Adresse beim Relay, die als QR-Code erscheint.
	StartURL string

	// ExpiresAt ist der Zeitpunkt, ab dem der Relay den Vorgang vergessen hat.
	ExpiresAt time.Time
}

// Status ist der Stand eines Verbindungsaufbaus.
type Status int

const (
	// StatusPending: Das Telefon hat die Anmeldung noch nicht abgeschlossen.
	StatusPending Status = iota
	// StatusConnected: Die Box ist mit Lightroom verbunden.
	StatusConnected
	// StatusExpired: Der Relay kennt den Vorgang nicht mehr; ein neuer
	// QR-Code ist nötig.
	StatusExpired
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusConnected:
		return "connected"
	case StatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// AccountInfo beschreibt die bestehende Verbindung.
//
// Nicht "Account": Der Name gehört dem Anwendungsfall, der sie liefert.
type AccountInfo struct {
	Connected bool
	// Name ist der Anzeigename des Adobe-Kontos, sofern Adobe ihn nennt.
	Name string
}

// Album ist ein Lightroom-Album.
type Album struct {
	ID   string
	Name string
	// CoverAssetID ist das Titelfoto; leer, wenn keins festgelegt ist.
	CoverAssetID string
}

// Asset ist ein Foto in Lightroom.
type Asset struct {
	ID       string
	FileName string
	// Width und Height sind die Maße nach dem Zuschnitt, sonst die des
	// Originals; 0, wenn Lightroom sie nicht nennt.
	Width, Height int
	// Rating sind die Sterne, 0 bis 5.
	Rating     int
	CapturedAt time.Time
}

// Page ist eine Seite Fotos.
type Page struct {
	Assets []Asset
	// Next ist die Marke für die nächste Seite; leer auf der letzten.
	Next string
}

// Photo ist ein heruntergeladenes Foto.
type Photo struct {
	// Name ist ein Dateiname für das Archiv, z. B. "IMG_0042.jpg".
	Name string
	// Data ist das JPEG.
	Data []byte
}

// assetJSON ist ein Foto, wie Lightroom es beschreibt.
//
// Alle Zahlen sind flexInt: Die Metadaten stammen aus Kameras, Telefonen und
// Importen über Jahre, und ein einzelnes Foto mit einer Breite als Text soll
// nicht die ganze Seite unlesbar machen.
type assetJSON struct {
	ID      string `json:"id"`
	Subtype string `json:"subtype"`
	Payload struct {
		CaptureDate  string `json:"captureDate"`
		ImportSource struct {
			FileName       string  `json:"fileName"`
			OriginalWidth  flexInt `json:"originalWidth"`
			OriginalHeight flexInt `json:"originalHeight"`
		} `json:"importSource"`
		Rating  *flexInt `json:"rating"`
		Ratings map[string]struct {
			Rating flexInt `json:"rating"`
		} `json:"ratings"`
		Develop struct {
			CroppedWidth  flexInt `json:"croppedWidth"`
			CroppedHeight flexInt `json:"croppedHeight"`
		} `json:"develop"`
	} `json:"payload"`
}

// isImage meldet, ob das Asset ein Foto ist. Ein fehlender Subtyp gilt als
// Foto, weil die Liste "alle Fotos" bereits serverseitig gefiltert ist.
func (a assetJSON) isImage() bool {
	return a.Subtype == "" || a.Subtype == "image"
}

func (a assetJSON) toAsset() Asset {
	p := a.Payload

	w, h := int(p.Develop.CroppedWidth), int(p.Develop.CroppedHeight)
	if w <= 0 || h <= 0 {
		w, h = int(p.ImportSource.OriginalWidth), int(p.ImportSource.OriginalHeight)
	}

	// Die Sterne stehen je nach Version direkt im Payload oder je Nutzer in
	// "ratings". Bei mehreren Einträgen zählt der höchste: Ein Foto, das
	// irgendwer mit fünf Sternen versehen hat, ist ein Kandidat für den Druck.
	rating := 0
	if p.Rating != nil {
		rating = int(*p.Rating)
	}

	for _, r := range p.Ratings {
		rating = max(rating, int(r.Rating))
	}

	return Asset{
		ID:         a.ID,
		FileName:   p.ImportSource.FileName,
		Width:      max(w, 0),
		Height:     max(h, 0),
		Rating:     min(max(rating, 0), 5),
		CapturedAt: parseCaptureDate(p.CaptureDate),
	}
}

// parseCaptureDate liest das Aufnahmedatum.
//
// Lightroom speichert es meist ohne Zeitzone, so wie die Kamera es kennt.
// Ohne Zone gilt die Ortszeit – eine Feier wird dort gedruckt, wo sie
// fotografiert wurde. "0000-00-00T00:00:00" steht für ein unbekanntes Datum.
func parseCaptureDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "0000") {
		return time.Time{}
	}

	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}

	for _, layout := range []string{"2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t
		}
	}

	return time.Time{}
}

// flexInt nimmt Zahlen, Zahlen als Text und null an.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(strings.TrimSpace(string(b)), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}

	*f = flexInt(v)

	return nil
}

// decodeResources dekodiert jede Ressource einzeln und überspringt, was sich
// nicht lesen lässt, statt die ganze Seite zu verwerfen.
func decodeResources[T any](raw []json.RawMessage) []T {
	out := make([]T, 0, len(raw))

	for _, r := range raw {
		var v T
		if err := json.Unmarshal(r, &v); err != nil {
			continue
		}

		out = append(out, v)
	}

	return out
}

// archiveName bildet aus dem Originalnamen einen JPEG-Dateinamen.
//
// Das Original ist oft ein RAW ("IMG_0042.CR3"); heruntergeladen wird aber
// immer die JPEG-Stufe. Den Namen zu behalten hilft, das Foto später in
// Lightroom wiederzufinden.
func archiveName(fileName, assetID string) string {
	name := strings.ReplaceAll(strings.TrimSpace(fileName), `\`, "/")
	name = path.Base(name)

	if name == "." || name == "/" || name == "" {
		return "lightroom-" + safeID(assetID) + ".jpg"
	}

	name = strings.TrimSuffix(name, path.Ext(name))
	if name == "" {
		return "lightroom-" + safeID(assetID) + ".jpg"
	}

	return name + ".jpg"
}

func safeID(id string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}

		return -1
	}, id)
}
