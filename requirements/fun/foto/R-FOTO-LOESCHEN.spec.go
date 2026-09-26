package foto

import "github.com/worldiety/speclink/spec"

var RFotoLoeschen = spec.Requirement{
	ID:         "R-FOTO-LOESCHEN",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Bilder endgültig entfernen",
	Text:       "Ein Bild MUSS sich samt gesicherter Datei endgültig entfernen lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "bilder-entfernen"},
	},
}
