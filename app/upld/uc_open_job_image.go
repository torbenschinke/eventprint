package upld

import (
	"fmt"
	"io"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/auth"
)

// OpenJobImage öffnet das Originalbild eines wartenden Auftrags.
//
// Der Aufrufer schließt den Reader.
type OpenJobImage func(subject auth.Subject, id JobID) (io.ReadCloser, error)

// NewOpenJobImage erzeugt den [OpenJobImage] Anwendungsfall.
func NewOpenJobImage(registry *Registry, openReader image.OpenReader) OpenJobImage {
	return func(subject auth.Subject, id JobID) (io.ReadCloser, error) {
		if err := subject.Audit(PermFetchImage); err != nil {
			return nil, err
		}

		job, ok := registry.Find(tokenOf(subject), id)
		if !ok {
			return nil, errUnknownJob()
		}

		// Das Bild liegt im Image-Subsystem und gehört keinem Nutzer, deshalb
		// als Systemnutzer. Die Berechtigung wurde oben bereits geprüft.
		reader, err := openReader(user.SU(), job.Image)
		if err != nil {
			return nil, fmt.Errorf("cannot open upload image: %w", err)
		}

		if reader.IsNone() {
			return nil, errUnknownJob()
		}

		return reader.Unwrap(), nil
	}
}
