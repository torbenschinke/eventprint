package archiv

import "github.com/worldiety/speclink/spec"

var RArchivPlatz = spec.Requirement{
	ID:         "R-ARCHIV-PLATZ",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Speicherplatz einsehen",
	Text:       "Die Betreuung MUSS sehen können, wie viel Speicherplatz insgesamt vorhanden ist, wie viel das Fotoarchiv belegt und wie viel auf das übrige System entfällt.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "speicherplatz-einsehen"},
	},
}
