package archiv

import "github.com/worldiety/speclink/spec"

var RArchivExport = spec.Requirement{
	ID:         "R-ARCHIV-EXPORT",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Fotoarchiv als eine Datei herunterladen",
	Text:       "Das gesamte Fotoarchiv MUSS sich als einzelne Datei herunterladen lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "archiv-weitergeben"},
	},
}
