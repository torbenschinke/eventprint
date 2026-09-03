package archiv

import "github.com/worldiety/speclink/spec"

var RArchivLoeschen = spec.Requirement{
	ID:         "R-ARCHIV-LOESCHEN",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Fotoarchiv nach Rückfrage löschen",
	Text:       "Das Fotoarchiv MUSS sich vollständig löschen lassen. Dem Löschen MUSS eine ausdrückliche, gesonderte Bestätigung vorausgehen, da die Bilder danach unwiederbringlich fort sind.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "archiv-freigeben"},
	},
}
