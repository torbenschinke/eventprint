package printing

// Settings beschreiben den Drucker.
type Settings struct {
	// Queue ist der Name der CUPS-Warteschlange, z. B. "CZ01". Leer bedeutet
	// Testbetrieb: Aufträge laufen durch, gedruckt wird nichts.
	Queue string `json:"queue,omitempty"`

	// PageSize ist die PPD-Bezeichnung des Papierformats; leer bedeutet
	// [CupsPageSize].
	PageSize string `json:"pageSize,omitempty"`

	// PrintSpeed steuert das Tempo des Thermokopfes; leer bedeutet
	// [SpeedLow]. Langsam färbt kräftiger, weil der Kopf länger auf jeder
	// Zeile verweilt.
	PrintSpeed string `json:"printSpeed,omitempty"`
}

const (
	// SpeedLow druckt langsamer und farbkräftiger.
	SpeedLow = "LowSpeed"

	// SpeedNormal druckt schneller, kann aber blasser wirken.
	SpeedNormal = "Normal"
)

// Speed liefert die wirksame Druckgeschwindigkeit.
func (s Settings) Speed() string {
	if s.PrintSpeed == "" {
		return SpeedLow
	}

	return s.PrintSpeed
}

// TestMode meldet, ob ohne Drucker gearbeitet wird.
func (s Settings) TestMode() bool { return s.Queue == "" }

// laminate übersetzt die Oberfläche in den Wert des Gutenprint-Treibers.
func (f Finish) laminate() string {
	if f == FinishMatte {
		return "Matte"
	}

	return "Glossy"
}
