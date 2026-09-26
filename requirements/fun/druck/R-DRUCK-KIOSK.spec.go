package druck

import "github.com/worldiety/speclink/spec"

var RDruckKiosk = spec.Requirement{
	ID:         "R-DRUCK-KIOSK",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Drucken im Kiosk",
	Text:       "Gäste MÜSSEN ein Foto mit einem Tipp in einem freigegebenen Kiosk-Layout in begrenzter Anzahl drucken können; im Kiosk MÜSSEN Bilder von Handy und Kamera je nach Einstellung sofort gedruckt werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "drucken-im-kiosk"},
	},
}
