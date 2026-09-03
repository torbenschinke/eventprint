package druck

import "github.com/worldiety/speclink/spec"

var RDruckKeinNachdruck = spec.Requirement{
	ID:         "R-DRUCK-KEIN-NACHDRUCK",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Kein Ausdruck ohne Auslösung",
	Text:       "Ein aufgegebener Druckauftrag MUSS beim Druckdienst zurückgenommen werden, damit kein Ausdruck ohne erneute Auslösung entsteht.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "kein-ungewollter-ausdruck"},
	},
}
