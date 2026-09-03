package netz

import "github.com/worldiety/speclink/spec"

var RNetzZustand = spec.Requirement{
	ID:         "R-NETZ-ZUSTAND",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Bestehende Funkverbindung erkennen",
	Text:       "Die Oberfläche MUSS zeigen, mit welchem Funknetz die Fotobox verbunden ist und wie gut der Empfang ist.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/netz.md", Anchor: "verbindung-erkennen"},
	},
}
