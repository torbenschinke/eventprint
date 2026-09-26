package druck

import "github.com/worldiety/speclink/spec"

var RDruckPapier = spec.Requirement{
	ID:         "R-DRUCK-PAPIER",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Papiervorrat",
	Text:       "Der verbleibende Papiervorrat MUSS sichtbar sein; ein neu eingelegtes Set MUSS sich melden lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "papiervorrat"},
	},
}
