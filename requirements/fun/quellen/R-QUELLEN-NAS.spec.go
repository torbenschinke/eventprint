package quellen

import "github.com/worldiety/speclink/spec"

var RQuellenNas = spec.Requirement{
	ID:         "R-QUELLEN-NAS",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Fotos vom NAS übernehmen",
	Text:       "Eine Netzwerkfreigabe im Heimnetz MUSS sich am Gerät einrichten lassen; ihre Ordner und Bilder MÜSSEN sich durchsuchen und übernehmen lassen, und gelesen werden darf nur die eingerichtete Freigabe.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/quellen.md", Anchor: "nas-als-quelle"},
	},
}
