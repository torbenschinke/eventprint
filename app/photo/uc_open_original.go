package photo

import (
	"fmt"
	"io"
	"os"

	"go.wdy.de/nago/application/permission"
)

// OpenOriginal öffnet die unveränderte Bilddatei eines Fotos.
//
// Gedruckt wird aus genau dieser Datei. Vorschau, Druck und Export lesen
// damit dieselbe Quelle – ein Ausdruck kann nicht anders aussehen als das,
// was die Mediathek zeigt, nur weil zwei Kopien auseinandergelaufen sind.
type OpenOriginal func(subject permission.Auditable, id ID) (io.ReadCloser, error)

// NewOpenOriginal erzeugt den [OpenOriginal] Anwendungsfall.
func NewOpenOriginal(repo Repository, originals Originals) OpenOriginal {
	return func(subject permission.Auditable, id ID) (io.ReadCloser, error) {
		if err := subject.Audit(PermOpenOriginal); err != nil {
			return nil, err
		}

		opt, err := repo.FindByID(id)
		if err != nil {
			return nil, fmt.Errorf("cannot load photo: %w", err)
		}

		if opt.IsNone() {
			return nil, fmt.Errorf("photo %s: %w", id, os.ErrNotExist)
		}

		return originals.Open(opt.Unwrap().File)
	}
}
