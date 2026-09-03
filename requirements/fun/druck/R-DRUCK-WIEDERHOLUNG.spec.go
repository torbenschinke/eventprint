package druck

import "github.com/worldiety/speclink/spec"

var RDruckWiederholung = spec.Requirement{
	ID:         "R-DRUCK-WIEDERHOLUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Gescheiterten Auftrag wiederholen",
	Text:       "Ein gescheiterter Druckauftrag MUSS sich wiederholen lassen, ohne dass ein zweiter Ausdruck desselben Bildes entsteht.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "auftrag-wiederholen"},
	},
}
