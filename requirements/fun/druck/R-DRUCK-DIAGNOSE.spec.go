package druck

import "github.com/worldiety/speclink/spec"

var RDruckDiagnose = spec.Requirement{
	ID:         "R-DRUCK-DIAGNOSE",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Zustand des Druckers ohne Terminal erkennen",
	Text:       "Der Zustand des Druckers MUSS in der Oberfläche erkennbar sein, einschließlich fehlender Warteschlange, angehaltenem Gerät und Meldungen des Geräts.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "zustand-des-druckers"},
	},
}
