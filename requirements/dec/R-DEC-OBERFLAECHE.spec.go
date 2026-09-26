package dec

import "github.com/worldiety/speclink/spec"

var RDecOberflaeche = spec.Requirement{
	ID:         "R-DEC-OBERFLAECHE",
	Kind:       spec.Decision,
	Discipline: spec.Technical,
	Status:     spec.Informative,
	Title:      "Die Oberfläche wird mit gift gezeichnet, nicht im Browser",
	Text:       "Die Oberfläche am Gerät wird mit gift direkt auf die GPU gezeichnet; es gibt keinen Browser und keinen lokalen Webserver mehr.",
	Rationale: `Ohne Chromium startet das Gerät schneller, braucht weniger Speicher und
kann nicht versehentlich eine Webseite verlassen. Ein Absturz des Browsers,
eine Wiederherstellungsfrage nach dem harten Ausschalten und die Ableitung der
öffentlichen Adresse aus der ersten Verbindung entfallen als Fehlerquellen.`,
	Consequences: `Die Oberfläche lässt sich nicht mehr aus der Ferne im Browser öffnen.
Oberflächentests laufen mit dem Test-Harness von gift statt mit Playwright.
Der Upload-Dienst im Internet bleibt eine Nago-Anwendung.`,
	Sources: []spec.Source{
		{Doc: "requirements/_sources/entscheidungen.md", Anchor: "oberfläche-ohne-browser"},
	},
}
