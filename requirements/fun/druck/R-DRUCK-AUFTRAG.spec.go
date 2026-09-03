// Package druck hält die Anforderungen rund um die Ausgabe auf Papier.
package druck

import "github.com/worldiety/speclink/spec"

var RDruckAuftrag = spec.Requirement{
	ID:         "R-DRUCK-AUFTRAG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Druckauftrag annehmen und im Hintergrund abarbeiten",
	Text:       "Ein Foto MUSS mit dem gewählten Layout sofort in die Warteschlange gestellt und im Hintergrund gedruckt werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "druckauftrag-erteilen"},
	},
}
