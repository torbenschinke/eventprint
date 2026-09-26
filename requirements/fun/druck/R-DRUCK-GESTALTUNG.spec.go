package druck

import "github.com/worldiety/speclink/spec"

var RDruckGestaltung = spec.Requirement{
	ID:         "R-DRUCK-GESTALTUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Gestaltung des Blattes",
	Text:       "Das Blatt MUSS sich in Format, Rahmen und Rahmenfarbe, Farbanmutung, Beschriftung, Datumsstempel und Oberfläche gestalten lassen; alle Formate teilen sich das eine Papier des Druckers.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "gestaltung-des-blattes"},
	},
}
