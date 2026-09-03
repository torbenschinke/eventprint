package netz

import "github.com/worldiety/speclink/spec"

var RNetzVerbinden = spec.Requirement{
	ID:         "R-NETZ-VERBINDEN",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Mit einem Funknetz verbinden",
	Text:       "Die Fotobox MUSS sich über die Oberfläche mit einem gewählten Funknetz verbinden lassen. Das Kennwort eines gesicherten Netzes MUSS verdeckt abgefragt werden, und ein abgelehntes Kennwort MUSS sich von einer sonst gescheiterten Verbindung unterscheiden lassen.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/netz.md", Anchor: "verbindung-herstellen"},
	},
}
