package druck

import "github.com/worldiety/speclink/spec"

var RDruckAbbruch = spec.Requirement{
	ID:         "R-DRUCK-ABBRUCH",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Auftrag abbrechen",
	Text:       "Ein noch nicht gedruckter Auftrag MUSS sich abbrechen lassen und darf danach nicht doch noch gedruckt werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "auftrag-abbrechen"},
	},
}
