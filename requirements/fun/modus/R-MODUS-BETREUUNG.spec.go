package modus

import "github.com/worldiety/speclink/spec"

var RModusBetreuung = spec.Requirement{
	ID:         "R-MODUS-BETREUUNG",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Betreuung im Kiosk nur mit PIN",
	Text:       "Im Kiosk MÜSSEN Einstellungen und Mediathek gesperrt sein; mit einer PIN MUSS sich die Betreuung befristet freischalten lassen, und Fehleingaben MÜSSEN das Raten ausbremsen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "betreuung-im-kiosk"},
	},
}
