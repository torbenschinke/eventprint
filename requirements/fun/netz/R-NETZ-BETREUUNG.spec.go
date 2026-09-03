package netz

import "github.com/worldiety/speclink/spec"

var RNetzBetreuung = spec.Requirement{
	ID:         "R-NETZ-BETREUUNG",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Funknetz nur durch die Betreuung wechseln",
	Text:       "Das Suchen, Anzeigen und Wechseln des Funknetzes MUSS der Betreuung vorbehalten sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/netz.md", Anchor: "nur-für-die-betreuung"},
	},
}
