package foto

import "github.com/worldiety/speclink/spec"

var RFotoDruckvorlage = spec.Requirement{
	ID:         "R-FOTO-DRUCKVORLAGE",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Druck aus den Originaldaten",
	Text:       "Das Bild, das der Drucker bekommt, MUSS aus derselben unveränderten Quelle stammen wie das, was die Mediathek zeigt.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "vorlage-für-den-druck"},
	},
}
