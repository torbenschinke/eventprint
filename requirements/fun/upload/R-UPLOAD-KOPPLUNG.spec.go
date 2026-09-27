package upload

import "github.com/worldiety/speclink/spec"

var RUploadKopplung = spec.Requirement{
	ID:         "R-UPLOAD-KOPPLUNG",
	Kind:       spec.Functional,
	Discipline: spec.Mixed,
	Status:     spec.Normative,
	Title:      "Fotobox mit dem Konto koppeln",
	Text:       "Registrierte, bestätigte Nutzer MÜSSEN Fotoboxen per Mailadresse und sechsstelligem, 30 Minuten gültigem Code selbst koppeln können; die Box DARF nicht erfahren, ob es das Konto gibt, das Zugangstoken MUSS automatisch ausgetauscht werden, und falsche Codes MÜSSEN nach wenigen Versuchen sperren.",
	Sources: []spec.Source{
		{Doc: "requirements/_sources/upload.md", Anchor: "fotobox-mit-dem-konto-koppeln"},
	},
}
