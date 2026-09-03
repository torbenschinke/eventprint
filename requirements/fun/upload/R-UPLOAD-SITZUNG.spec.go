// Package upload hält die Anforderungen des Upload-Relais im Internet.
package upload

import "github.com/worldiety/speclink/spec"

var RUploadSitzung = spec.Requirement{
	ID:         "R-UPLOAD-SITZUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Kurzlebige Upload-Adresse je Fotobox",
	Text:       "Eine angemeldete Fotobox MUSS eine kurzlebige Upload-Adresse erhalten; je Fotobox darf höchstens eine Adresse gültig sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "upload-sitzung"},
	},
}
