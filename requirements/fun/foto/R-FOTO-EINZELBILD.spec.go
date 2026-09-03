package foto

import "github.com/worldiety/speclink/spec"

var RFotoEinzelbild = spec.Requirement{
	ID:         "R-FOTO-EINZELBILD",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Einzelnes Bild anhand seiner Kennung finden",
	Text:       "Ein einzelnes Bild MUSS anhand seiner Kennung auffindbar sein, damit ein Nachdruck ohne Durchsuchen der Historie möglich ist.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "einzelnes-bild"},
	},
}
