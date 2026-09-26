package archiv

import "github.com/worldiety/speclink/spec"

var RArchivLoeschen = spec.Requirement{
	ID:         "R-ARCHIV-LOESCHEN",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Feier abschließen",
	Text:       "Die Fotos einer Feier MÜSSEN sich gesammelt löschen lassen; private Fotos DÜRFEN davon nicht betroffen sein.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "feier-abschließen"},
	},
}
