package printing

import (
	"github.com/worldiety/enum"
	"go.wdy.de/nago/application/settings"
)

// PrinterSource ist der Name des Kontextwerts, unter dem die verfügbaren
// CUPS-Warteschlangen bereitstehen. Das Einstellungsformular macht daraus
// eine Auswahlliste, statt den Namen abtippen zu lassen.
const PrinterSource = "eventprint.printers"

// Settings sind die global gültigen Druckereinstellungen.
//
// Sie werden über das Admin-Center gepflegt, damit die Fotobox vor Ort ohne
// Umgebungsvariablen und ohne Neustart eingerichtet werden kann. Der
// Zero-Wert ist gültig und bedeutet Testbetrieb – das ist Vorgabe des
// Settings-Vertrags von Nago und hier auch die sichere Voreinstellung: Ohne
// bewusste Auswahl wird kein Papier verbraucht.
type Settings struct {
	_ any `title:"Fotodrucker" description:"Ziel und Papierformat der Ausdrucke."`

	// Queue ist der Name der CUPS-Warteschlange, z. B. "CZ01".
	Queue string `json:"queue,omitempty" label:"Warteschlange" source:"eventprint.printers" supportingText:"Leer bedeutet Testbetrieb: Aufträge laufen durch, es wird nichts gedruckt."`

	// PageSize ist die PPD-Bezeichnung des Papierformats. Leer bedeutet
	// [CupsPageSize], also 10x15 cm.
	PageSize string `json:"pageSize,omitempty" label:"Papierformat" supportingText:"PPD-Bezeichnung, Standard w288h432 für 10x15 cm."`

	// Laminate ist das Oberflächenfinish des Schutzüberzugs.
	Laminate string `json:"laminate,omitempty" label:"Oberfläche" values:"[\"Glossy\",\"Matte\",\"PartialMatte\"]" supportingText:"Glänzend, matt oder seidenmatt. Der CZ-01 trägt den Überzug beim Druck auf."`

	// PrintSpeed steuert das Tempo des Thermokopfes. Leer bedeutet
	// [SpeedLow], siehe [Settings.Speed].
	PrintSpeed string `json:"printSpeed,omitempty" label:"Druckgeschwindigkeit" values:"[\"LowSpeed\",\"Normal\"]" supportingText:"Langsam färbt kräftiger, weil der Thermokopf länger auf jeder Zeile verweilt. Normal ist schneller, kann aber blasser wirken."`
}

// Werte für [Settings.PrintSpeed], wie sie der Gutenprint-Treiber erwartet.
const (
	// SpeedLow lässt den Thermokopf langsamer laufen. Jede Zeile bekommt
	// dadurch mehr Zeit, die Farbe wird kräftiger und weniger verwaschen.
	SpeedLow = "LowSpeed"

	// SpeedNormal ist die Vorgabe des Treibers.
	SpeedNormal = "Normal"
)

// Speed liefert die einzustellende Druckgeschwindigkeit.
//
// Ohne ausdrückliche Wahl wird bewusst langsam gedruckt: Für eine Fotobox
// zählt das Ergebnis mehr als der Durchsatz, und bei normaler Geschwindigkeit
// wirken die Farben auf diesem Gerät sichtbar blasser. Wer den Durchsatz
// braucht, stellt in den Einstellungen auf Normal.
func (s Settings) Speed() string {
	if s.PrintSpeed == "" {
		return SpeedLow
	}

	return s.PrintSpeed
}

func (Settings) GlobalSettings() bool { return true }

// Registrierung im offenen Summentyp der globalen Einstellungen. Ohne diese
// Zeile taucht die Seite im Admin-Center nicht auf.
//
// Der Name muss zwingend vergeben werden: enum nutzt sonst den bloßen
// Go-Typnamen als Diskriminator im JSON. Da "Settings" mehrfach vorkommt –
// auch in Nago selbst – würden die Einstellungen sonst gegenseitig
// überschrieben und beim Lesen als falscher Typ ausgepackt.
var _ = enum.Variant[settings.GlobalSettings, Settings](
	enum.Rename[Settings]("eventprint.printer.settings"),
)

// TestMode meldet, ob mangels Warteschlange nur simuliert wird.
func (s Settings) TestMode() bool { return s.Queue == "" }
