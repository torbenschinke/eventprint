// Package modus hält die Anforderungen an die Betriebsarten Heimbetrieb und Kiosk.
package modus

import "github.com/worldiety/speclink/spec"

var RModusHeim = spec.Requirement{
	ID:         "R-MODUS-HEIM",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Heimbetrieb nach dem Einschalten",
	Text:       "Nach jedem Einschalten MUSS das Gerät im Heimbetrieb starten; ein Neustart allein der Anwendung darf einen laufenden Kiosk nicht beenden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/modus.md", Anchor: "heimbetrieb-nach-dem-start"},
	},
}
