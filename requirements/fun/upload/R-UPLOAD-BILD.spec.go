package upload

import "github.com/worldiety/speclink/spec"

var RUploadBild = spec.Requirement{
	ID:         "R-UPLOAD-BILD",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Originalbild eines wartenden Auftrags laden",
	Text:       "Zu einem wartenden Auftrag MUSS das Originalbild abrufbar sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "bild-eines-auftrags-laden"},
	},
}
