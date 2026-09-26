package photo

import (
	"go.wdy.de/nago/application/permission"
)

// Location ist der Ablageort eines Originals.
type Location struct {
	Photo Photo

	// Path ist der absolute Pfad der Datei.
	Path string

	// ExportName ist der Name, unter dem das Foto weitergegeben wird.
	ExportName string
}

// Locate liefert die Ablageorte der Originale, etwa für Vorschaubilder oder
// den Export auf einen USB-Stick.
//
// Verschwundene Fotos werden übergangen; eine Auswahl von vorhin darf nicht
// daran scheitern, dass inzwischen eines gelöscht wurde.
type Locate func(subject permission.Auditable, ids ...ID) ([]Location, error)

// NewLocate erzeugt den [Locate] Anwendungsfall.
func NewLocate(repo Repository, originals Originals) Locate {
	return func(subject permission.Auditable, ids ...ID) ([]Location, error) {
		if err := subject.Audit(PermLocate); err != nil {
			return nil, err
		}

		out := make([]Location, 0, len(ids))
		for _, id := range ids {
			opt, err := repo.FindByID(id)
			if err != nil {
				return nil, err
			}

			if opt.IsNone() {
				continue
			}

			p := opt.Unwrap()

			// Wer nicht die ganze Mediathek sehen darf, bekommt private Fotos
			// auch dann nicht, wenn er ihre Kennung kennt. Im Kiosk kennt die
			// Oberfläche nur Fotos der Feier; diese Prüfung ist der Riegel
			// dafür, dass das nicht an der Oberfläche allein hängt.
			if !visibleTo(subject, p) {
				continue
			}

			out = append(out, Location{Photo: p, Path: originals.Path(p.File), ExportName: ExportName(p)})
		}

		return out, nil
	}
}

// ExportName bildet den Dateinamen für die Weitergabe.
//
// Der Aufnahmezeitpunkt steht vorn, damit ein Ordner auf dem Stick in jedem
// Dateimanager chronologisch sortiert erscheint; der ursprüngliche Name folgt,
// damit ein Gast sein eigenes Bild wiederfindet.
func ExportName(p Photo) string {
	stamp := p.CreatedAt.Local().Format("2006-01-02_15-04-05")
	if p.Name == "" {
		return stamp + "_" + p.File
	}

	return stamp + "_" + p.Name
}
