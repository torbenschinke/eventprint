package modus

import "github.com/worldiety/speclink/spec"

var RModusPrivat = spec.Requirement{
	ID:         "R-MODUS-PRIVAT",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Gäste sehen nur die Fotos der Feier",
	Text:       "Im Kiosk DÜRFEN Gäste ausschließlich die Fotos der laufenden Feier sehen und drucken; private Fotos und die anderer Feiern MÜSSEN verborgen bleiben, auch bei bekannter Kennung.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "feier-und-mediathek-getrennt"},
	},
}
