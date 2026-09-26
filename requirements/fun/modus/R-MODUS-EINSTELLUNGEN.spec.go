package modus

import "github.com/worldiety/speclink/spec"

var RModusEinstellungen = spec.Requirement{
	ID:         "R-MODUS-EINSTELLUNGEN",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Einstellungen am Gerät",
	Text:       "Alle Einstellungen MÜSSEN sich am Gerät selbst vornehmen lassen, ohne Terminal und ohne zweiten Rechner.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "einstellungen-am-gerät"},
	},
}
