package photo

import (
	"errors"

	"go.wdy.de/nago/application/permission"
)

// FindEvent liefert die Fotos genau einer Feier, die neuesten zuerst.
//
// Ein eigener Anwendungsfall und kein Filter von [FindAll]: Die Schranke
// zwischen Feier und Mediathek soll an der Berechtigung hängen und nicht daran,
// dass eine Oberfläche den richtigen Filter setzt.
type FindEvent func(subject permission.Auditable, event EventID, limit int) ([]Photo, error)

// NewFindEvent erzeugt den [FindEvent] Anwendungsfall.
func NewFindEvent(repo Repository) FindEvent {
	return func(subject permission.Auditable, event EventID, limit int) ([]Photo, error) {
		if err := subject.Audit(PermFindEvent); err != nil {
			return nil, err
		}

		// Ohne Feier wäre die Abfrage "alle privaten Fotos" – genau das,
		// was dieser Anwendungsfall verhindern soll.
		if event == "" {
			return nil, errors.New("es läuft keine Feier")
		}

		return query(repo, Query{Scope: ScopeEvent, Event: event, Limit: limit})
	}
}
