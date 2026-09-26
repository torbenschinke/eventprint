package modus

import "github.com/worldiety/speclink/spec"

var RModusKiosk = spec.Requirement{
	ID:         "R-MODUS-KIOSK",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Kiosk bis zum nächsten Einschalten, mit Vorabprüfung",
	Text:       "Der Besitzer MUSS das Gerät in den Kiosk versetzen können, der bis zum nächsten Einschalten gilt; vorher MUSS das Gerät Drucker, Papier, Upload-Dienst und Kamera prüfen und das Ergebnis zeigen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "kiosk-starten"},
	},
}
