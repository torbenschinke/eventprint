// Package archiv hält die Anforderungen an die Weitergabe und den Speicherplatz.
package archiv

import "github.com/worldiety/speclink/spec"

var RArchivExport = spec.Requirement{
	ID:         "R-ARCHIV-EXPORT",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Fotos auf einen USB-Stick kopieren",
	Text:       "Die Fotos einer Feier, eine Auswahl oder alle Fotos MÜSSEN sich im Original auf einen USB-Stick kopieren lassen; ein Abbruch MUSS sich ohne doppelte Dateien wiederholen lassen, und der Stick MUSS sich sicher auswerfen lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/archiv.md", Anchor: "fotos-weitergeben"},
	},
}
