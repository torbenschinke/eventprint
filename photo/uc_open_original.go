package photo

import (
	"io"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
)

// NewOpenOriginal erzeugt den [OpenOriginal] Anwendungsfall.
//
// Für den Druck werden bewusst die Originaldaten und nicht eine der
// verkleinerten SrcSet-Varianten verwendet, damit die vollen 300 dpi des
// Dye-Sublimation-Druckers ausgenutzt werden.
func NewOpenOriginal(findByID FindByID, openReader image.OpenReader) OpenOriginal {
	return func(subject auth.Subject, id ID) (option.Opt[io.ReadCloser], error) {
		optPhoto, err := findByID(subject, id)
		if err != nil {
			return option.Opt[io.ReadCloser]{}, err
		}

		if optPhoto.IsNone() {
			return option.Opt[io.ReadCloser]{}, nil
		}

		return openReader(subject, optPhoto.Unwrap().Image)
	}
}
