package druck

import "github.com/worldiety/speclink/spec"

var RDruckVorschau = spec.Requirement{
	ID:         "R-DRUCK-VORSCHAU",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Vorschau vor dem Druck",
	Text:       "Vor dem Druck MUSS das Ergebnis des gewählten Layouts als Bild sichtbar sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "vorschau-des-ergebnisses"},
	},
}
