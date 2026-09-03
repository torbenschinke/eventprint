// Package foto hält die Anforderungen rund um die Bilder der Fotobox.
package foto

import "github.com/worldiety/speclink/spec"

var RFotoImport = spec.Requirement{
	ID:         "R-FOTO-IMPORT",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Eingehende Bilder aufnehmen und im Original sichern",
	Text:       "Ein eingehendes Bild MUSS unabhängig von seiner Herkunft aufgenommen und dabei unverändert gesichert werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "bilder-aufnehmen"},
	},
}
