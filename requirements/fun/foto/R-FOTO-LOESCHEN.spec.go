package foto

import "github.com/worldiety/speclink/spec"

var RFotoLoeschen = spec.Requirement{
	ID:         "R-FOTO-LOESCHEN",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Bild aus der Historie entfernen",
	Text:       "Ein Bild MUSS sich aus der Historie entfernen lassen, damit eine Fehlaufnahme nicht den ganzen Abend sichtbar bleibt.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "bilder-entfernen"},
	},
}
