package upload

import "github.com/worldiety/speclink/spec"

var RUploadAbholung = spec.Requirement{
	ID:         "R-UPLOAD-ABHOLUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Wartende Aufträge abholen",
	Text:       "Eine Fotobox MUSS die für sie hinterlegten Aufträge abrufen können und dabei ausschließlich ihre eigenen sehen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "wartende-aufträge-abholen"},
	},
}
