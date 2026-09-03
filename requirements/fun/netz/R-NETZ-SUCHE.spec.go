package netz

import "github.com/worldiety/speclink/spec"

var RNetzSuche = spec.Requirement{
	ID:         "R-NETZ-SUCHE",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Verfügbare Funknetze auflisten",
	Text:       "Die verfügbaren Funknetze MÜSSEN sich auflisten lassen. Ein Funknetz MUSS genau einmal erscheinen, auch wenn es über mehrere Zugangspunkte oder Frequenzbänder empfangen wird.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/netz.md", Anchor: "netze-finden"},
	},
}
