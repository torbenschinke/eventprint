package photo

import (
	"io"

	"github.com/worldiety/option"
	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/auth"
)

// OpenOriginal öffnet die unveränderten Originaldaten eines Fotos, so wie sie
// von Kamera oder Smartphone geliefert wurden.
type OpenOriginal func(subject auth.Subject, id ID) (option.Opt[io.ReadCloser], error)

// NewOpenOriginal erzeugt den [OpenOriginal] Anwendungsfall.
//
// Für den Druck werden bewusst die Originaldaten und nicht eine der
// verkleinerten SrcSet-Varianten verwendet, damit die vollen 300 dpi des
// Dye-Sublimation-Druckers ausgenutzt werden.
func NewOpenOriginal(findByID FindByID, openReader image.OpenReader) OpenOriginal {
	return func(subject auth.Subject, id ID) (option.Opt[io.ReadCloser], error) {
		// Die Originaldaten sind mehr als die Anzeige: Sie tragen den
		// EXIF-Block mit Aufnahmezeit und Gerät. Wer ein Vorschaubild sehen
		// darf, darf deshalb nicht zwingend auch das hier.
		if err := subject.Audit(PermOpenOriginal); err != nil {
			return option.Opt[io.ReadCloser]{}, err
		}

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
