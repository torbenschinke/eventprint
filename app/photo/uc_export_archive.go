package photo

import (
	"io"
	"os"

	"go.wdy.de/nago/auth"
)

// ExportArchive schreibt das gesamte Fotoarchiv als ZIP nach dst.
//
// Strömend und nicht in den Speicher: Das Archiv einer Feier wiegt mehrere
// Gigabyte, und die Fotobox hat vier davon insgesamt.
type ExportArchive func(subject auth.Subject, dst io.Writer) (int, error)

// NewExportArchive erzeugt den [ExportArchive] Anwendungsfall.
func NewExportArchive(dir string) ExportArchive {
	return func(subject auth.Subject, dst io.Writer) (int, error) {
		if err := subject.Audit(PermExportArchive); err != nil {
			return 0, err
		}

		return writeArchiveZip(os.DirFS(dir), dst)
	}
}
