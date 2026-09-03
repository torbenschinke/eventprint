package foto

import "github.com/worldiety/speclink/spec"

var RFotoDruckvorlage = spec.Requirement{
	ID:         "R-FOTO-DRUCKVORLAGE",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Originaldaten als Vorlage für den Druck",
	Text:       "Der Druck MUSS aus den unveränderten Originaldaten desselben Bildes erfolgen, das die Historie zeigt.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "vorlage-für-den-druck"},
	},
}
