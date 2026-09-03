// Package dec hält die Festlegungen, die nicht aus einer Anforderung folgen.
package dec

import "github.com/worldiety/speclink/spec"

var RDecZustandsablage = spec.Requirement{
	ID:         "R-DEC-ZUSTANDSABLAGE",
	Kind:       spec.Decision,
	Discipline: spec.Technical,
	Status:     spec.Normative,
	Title:      "Aggregate werden als Zustand abgelegt, nicht als Ereignisfolge",
	Text:       "Foto und Druckauftrag MÜSSEN als aktueller Zustand gespeichert werden; ihr Verlauf wird nicht als Folge von Ereignissen aufbewahrt.",
	Rationale: `Eine Fotobox läuft einen Abend lang. Gefragt ist, ob das Bild auf Papier
ist, nicht, in welcher Reihenfolge ein Auftrag seine Zustände durchlaufen hat.
Der Zustand passt in eine JSON-Ablage, die sich ohne Werkzeug lesen und im
Zweifel von Hand reparieren lässt – auf einer Feier um Mitternacht ist das der
entscheidende Vorteil.`,
	Consequences: `Der Verlauf ist unwiederbringlich verloren: Warum ein Auftrag zweimal
gescheitert ist, lässt sich hinterher nicht mehr rekonstruieren, und genau das
hat die Suche nach den ungewollten Nachdrucken erschwert. Eine spätere
Auswertung über mehrere Veranstaltungen hinweg ist aus diesen Daten nicht zu
gewinnen. Die Umstellung wäre nachträglich teuer, weil bestehende Daten keine
Ereignisse enthalten, aus denen sich ein Verlauf bilden ließe.`,
	Sources: []spec.Source{
		{Doc: "requirements/_sources/entscheidungen.md", Anchor: "form-der-ablage"},
	},
}
