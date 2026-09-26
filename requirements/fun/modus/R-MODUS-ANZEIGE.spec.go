package modus

import "github.com/worldiety/speclink/spec"

var RModusAnzeige = spec.Requirement{
	ID:         "R-MODUS-ANZEIGE",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Bedienbar auf jedem Touchscreen",
	Text:       "Die Oberfläche MUSS auf Touchscreens von 800 × 480 bis Full-HD vollständig bedienbar sein und sich nach Auflösung und Größe des Panels bemessen; das Erscheinungsbild MUSS sich hell, dunkel oder nach der Tageszeit wählen lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "bildschirme"},
	},
}
