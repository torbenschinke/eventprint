package druck

import "github.com/worldiety/speclink/spec"

var RDruckStatus = spec.Requirement{
	ID:         "R-DRUCK-STATUS",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Zustand der Druckaufträge einsehen",
	Text:       "Alle Druckaufträge MÜSSEN mit Zustand und Fehlerursache abrufbar sein, vollständig wie auch einzeln anhand ihrer Kennung.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "zustand-der-aufträge"},
	},
}
