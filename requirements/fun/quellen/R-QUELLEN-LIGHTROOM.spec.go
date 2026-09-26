package quellen

import "github.com/worldiety/speclink/spec"

var RQuellenLightroom = spec.Requirement{
	ID:         "R-QUELLEN-LIGHTROOM",
	Kind:       spec.Functional,
	Discipline: spec.Business,
	Status:     spec.Normative,
	Title:      "Lightroom durchsuchen und bearbeitete Fassung drucken",
	Text:       "Alben und Fotos aus Lightroom MÜSSEN sich am Gerät durchsuchen lassen; gedruckt werden MUSS die in Lightroom bearbeitete Fassung.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/quellen.md", Anchor: "lightroom-durchsuchen"},
	},
}
