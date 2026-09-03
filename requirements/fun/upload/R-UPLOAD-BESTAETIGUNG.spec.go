package upload

import "github.com/worldiety/speclink/spec"

var RUploadBestaetigung = spec.Requirement{
	ID:         "R-UPLOAD-BESTAETIGUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Auftrag erst nach Bestätigung löschen",
	Text:       "Ein Auftrag MUSS beim Dienst erhalten bleiben, bis die Fotobox seine Übernahme bestätigt hat.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "übernahme-bestätigen"},
	},
}
