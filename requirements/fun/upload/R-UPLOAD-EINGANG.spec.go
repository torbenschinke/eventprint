package upload

import "github.com/worldiety/speclink/spec"

var RUploadEingang = spec.Requirement{
	ID:         "R-UPLOAD-EINGANG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Upload mehrerer Bilder in den Eingang",
	Text:       "Im Heimbetrieb MUSS die Upload-Seite mehrere Bilder auf einmal ohne Abfrage der Gestaltung annehmen; welche Art von Upload gemeint ist, MUSS die Adresse im QR-Code bestimmen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "upload-in-den-eingang"},
	},
}
