package foto

import "github.com/worldiety/speclink/spec"

var RFotoEingang = spec.Requirement{
	ID:         "R-FOTO-EINGANG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Eingang im Heimbetrieb",
	Text:       "Im Heimbetrieb MÜSSEN Bilder von Handy und Kamera im Eingang landen, ohne von allein gedruckt zu werden, und als neu gelten, bis sie für den Druck ausgewählt werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/foto.md", Anchor: "eingang"},
	},
}
