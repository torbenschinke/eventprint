package foto

import "github.com/worldiety/speclink/spec"

var RFotoHistorie = spec.Requirement{
	ID:         "R-FOTO-HISTORIE",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Mediathek mit Favoriten und gedruckten Fotos",
	Text:       "Alle Fotos MÜSSEN in einer Mediathek sichtbar sein, die neuesten zuerst; Fotos MÜSSEN sich als Favorit markieren lassen, gedruckte Fotos MÜSSEN erkennbar sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "mediathek"},
	},
}
