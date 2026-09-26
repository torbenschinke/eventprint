// Package quellen hält die Anforderungen an weitere Bildquellen: Adobe Lightroom und USB-Stick.
package quellen

import "github.com/worldiety/speclink/spec"

var RQuellenLightroomAnmeldung = spec.Requirement{
	ID:         "R-QUELLEN-LIGHTROOM-ANMELDUNG",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Lightroom per Handy verbinden",
	Text:       "Das Lightroom-Konto MUSS sich verbinden lassen, ohne am Gerät ein Kennwort einzugeben; die Rückmeldung der Anmeldung MUSS über den Upload-Dienst laufen, und die Verbindung MUSS sich trennen lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/quellen.md", Anchor: "lightroom-verbinden"},
	},
}
