package druck

import "github.com/worldiety/speclink/spec"

var RDruckFreigabe = spec.Requirement{
	ID:         "R-DRUCK-FREIGABE",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Angehaltenen Drucker ohne Terminal freigeben",
	Text:       "Hält der Druckdienst den Drucker an, MUSS die Fotobox ihn selbsttätig wieder freigeben, und die Betreuung MUSS ihn in der Oberfläche sofort freigeben können. Solange er angehalten ist, DARF ein wartender Auftrag nicht wegen Zeitüberschreitung verworfen werden.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/druck.md", Anchor: "drucker-freigeben"},
	},
}
