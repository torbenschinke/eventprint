package device

import (
	"time"

	"github.com/torbenschinke/eventprint/app/nas"
	"github.com/torbenschinke/eventprint/app/photo"
	"github.com/torbenschinke/eventprint/app/printing"
)

// Kiosk ist der Zustand einer laufenden Feier.
//
// Er wird ausschließlich im flüchtigen Laufzeitverzeichnis abgelegt. Ein
// Neustart der Anwendung – etwa nach einem Absturz – findet ihn dort wieder
// und bleibt im Kiosk; ein Neustart des Geräts, also Stecker ziehen und
// wieder einstecken, löscht ihn und führt in den Heimbetrieb. Genau diese
// Unterscheidung ist gewollt: Ein Absturz darf Gästen nicht die private
// Mediathek öffnen, ein bewusstes Ausschalten soll es.
type Kiosk struct {
	Event photo.EventID `json:"event"`
	Title string        `json:"title"`
	Since time.Time     `json:"since"`

	// Die Gestaltung der Feier wird beim Start festgehalten. Gäste dürfen die
	// Einstellungen nicht lesen – darin stehen Zugangstoken und PIN –, und
	// ändern kann sie während der Feier ohnehin niemand.
	Accent    string                `json:"accent"`
	Layouts   []printing.TemplateID `json:"layouts"`
	MaxCopies int                   `json:"maxCopies"`
}

// Active meldet, ob eine Feier läuft.
func (k Kiosk) Active() bool { return k.Event != "" }

// Event ist eine vergangene oder laufende Feier.
type Event struct {
	ID        photo.EventID `json:"id"`
	Title     string        `json:"title"`
	StartedAt time.Time     `json:"startedAt"`
}

// Settings sind die dauerhaften Einstellungen des Geräts.
type Settings struct {
	// EventTitle ist die Überschrift der nächsten Feier.
	EventTitle string `json:"eventTitle,omitempty"`

	// Accent ist die Akzentfarbe des Kiosks als #RRGGBB.
	Accent string `json:"accent,omitempty"`

	// KioskLayouts sind die Layouts, zwischen denen Gäste wählen.
	KioskLayouts []printing.TemplateID `json:"kioskLayouts,omitempty"`

	// KioskDefault ist das Layout für Aufträge ohne Wahl, etwa von der Kamera.
	KioskDefault printing.TemplateID `json:"kioskDefault,omitempty"`

	// MaxCopies begrenzt, wie oft ein Gast ein Foto auf einmal druckt.
	MaxCopies int `json:"maxCopies,omitempty"`

	// PrintUploads druckt Handy-Uploads im Kiosk sofort.
	PrintUploads bool `json:"printUploads"`

	// PrintCamera druckt Kameraaufnahmen im Kiosk sofort.
	PrintCamera bool `json:"printCamera"`

	// Pin ist die Betreuer-PIN in abgeleiteter Form.
	Pin PinHash `json:"pin,omitzero"`

	// RelayURL und RelayToken verbinden das Gerät mit dem öffentlichen
	// Upload-Dienst.
	RelayURL   string `json:"relayUrl,omitempty"`
	RelayToken string `json:"relayToken,omitempty"`

	// RelayAccount ist die Mailadresse des Kontos, mit dem die Box
	// gekoppelt wurde; leer, wenn das Token von Hand eingetragen ist.
	RelayAccount string `json:"relayAccount,omitempty"`

	// Appearance ist das Erscheinungsbild im Heimbetrieb: hell, dunkel oder
	// nach der Tageszeit. Der Kiosk ist immer dunkel.
	Appearance Appearance `json:"appearance,omitempty"`

	// ReduceTransparency macht die Glasflächen im Heimbetrieb deckend.
	ReduceTransparency bool `json:"reduceTransparency,omitempty"`

	// GlassTint ist die Tönung des Glases in Prozent, von klar (1) bis
	// getönt (100), wie der Regler von iOS 27. Null heißt Voreinstellung.
	GlassTint int `json:"glassTint,omitempty"`

	// NAS ist die Netzwerkfreigabe mit den Fotos des Haushalts.
	NAS nas.Config `json:"nas,omitzero"`

	// Printer beschreibt den Drucker.
	Printer printing.Settings `json:"printer,omitzero"`

	// PaperLeft ist der geschätzte Papiervorrat. Der CZ-01 meldet ihn nicht
	// zuverlässig; das Gerät zählt deshalb selbst mit, und wer ein neues Set
	// einlegt, sagt es.
	PaperLeft int `json:"paperLeft"`

	// PaperCapacity ist die Größe eines Papiersets.
	PaperCapacity int `json:"paperCapacity,omitempty"`

	// FaceCrop richtet Ausschnitte an erkannten Gesichtern aus.
	FaceCrop bool `json:"faceCrop"`

	// Events sind die bisherigen Feiern, die jüngste zuletzt.
	Events []Event `json:"events,omitempty"`
}

// DefaultSettings sind die Werte eines frisch eingerichteten Geräts.
func DefaultSettings() Settings {
	return Settings{
		Accent:        "#E9B949",
		KioskLayouts:  []printing.TemplateID{printing.TemplateFull, printing.TemplatePassepartout, printing.TemplatePolaroid},
		KioskDefault:  printing.TemplatePolaroid,
		MaxCopies:     2,
		PrintUploads:  true,
		PrintCamera:   true,
		PaperCapacity: 50,
		PaperLeft:     50,
		FaceCrop:      true,
	}
}

// Normalized ergänzt fehlende Werte, damit ältere Einstellungsdateien und
// halb ausgefüllte Formulare nie zu einem unbenutzbaren Gerät führen.
func (s Settings) Normalized() Settings {
	d := DefaultSettings()
	if s.Accent == "" {
		s.Accent = d.Accent
	}

	if len(s.KioskLayouts) == 0 {
		s.KioskLayouts = d.KioskLayouts
	}

	if s.KioskDefault == "" {
		s.KioskDefault = d.KioskDefault
	}

	if s.MaxCopies <= 0 {
		s.MaxCopies = d.MaxCopies
	}

	if s.PaperCapacity <= 0 {
		s.PaperCapacity = d.PaperCapacity
	}

	return s
}

// Event liefert die Feier zur Kennung.
func (s Settings) Event(id photo.EventID) (Event, bool) {
	for _, e := range s.Events {
		if e.ID == id {
			return e, true
		}
	}

	return Event{}, false
}

// SettingsStore hält die Einstellungen.
type SettingsStore interface {
	Load() (Settings, error)
	Save(Settings) error
}

// KioskStore hält den Zustand der laufenden Feier.
type KioskStore interface {
	Load() (Kiosk, error)
	Save(Kiosk) error
	Clear() error
}

// Appearance ist das Erscheinungsbild der Oberfläche.
type Appearance string

const (
	// AppearanceAuto ist abends und nachts dunkel, tagsüber hell. Die Box
	// hat keinen Lichtsensor; die Uhrzeit ist die beste Näherung dafür, ob
	// das Zimmer dunkel ist.
	AppearanceAuto  Appearance = ""
	AppearanceLight Appearance = "light"
	AppearanceDark  Appearance = "dark"
)

// Dark meldet, ob zur Uhrzeit t dunkel gezeichnet wird.
func (a Appearance) Dark(t time.Time) bool {
	switch a {
	case AppearanceLight:
		return false
	case AppearanceDark:
		return true
	default:
		h := t.Hour()
		return h >= 20 || h < 7
	}
}
