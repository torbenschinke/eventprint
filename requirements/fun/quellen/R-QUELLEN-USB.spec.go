package quellen

import "github.com/worldiety/speclink/spec"

var RQuellenUsb = spec.Requirement{
	ID:         "R-QUELLEN-USB",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Bilder vom USB-Stick übernehmen",
	Text:       "Bilder auf einem USB-Stick MÜSSEN sich durchsuchen und übernehmen lassen; gelesen werden darf nur der Stick.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/quellen.md", Anchor: "usb-stick-als-quelle"},
	},
}
