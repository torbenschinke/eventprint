package foto

import "github.com/worldiety/speclink/spec"

var RFotoHistorie = spec.Requirement{
	ID:         "R-FOTO-HISTORIE",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Historie der Bilder, die neuesten zuerst",
	Text:       "Die entstandenen Bilder MÜSSEN abrufbar sein, beginnend mit dem neuesten; wahlweise vollständig oder auf die jüngsten begrenzt.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "historie"},
	},
}
