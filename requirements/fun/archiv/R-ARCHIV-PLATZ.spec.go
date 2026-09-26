package archiv

import "github.com/worldiety/speclink/spec"

var RArchivPlatz = spec.Requirement{
	ID:         "R-ARCHIV-PLATZ",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Speicherplatz einsehen",
	Text:       "Es MUSS sichtbar sein, wie viel Platz die Fotos belegen und wie viel frei ist.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "speicherplatz-einsehen"},
	},
}
