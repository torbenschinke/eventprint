package printing

import (
	"fmt"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"

	"github.com/torbenschinke/eventprint/photo"
)

// NewPreview erzeugt den [Preview] Anwendungsfall. Er rendert exakt dasselbe
// Bild wie der Druck-Worker, sodass die Vorschau verbindlich ist.
func NewPreview(openOriginal photo.OpenOriginal, raster func() Raster) Preview {
	return func(subject auth.Subject, id photo.ID, tpl TemplateID) ([]byte, error) {
		if err := subject.Audit(PermPrint); err != nil {
			return nil, err
		}

		optReader, err := openOriginal(subject, id)
		if err != nil {
			return nil, err
		}

		if optReader.IsNone() {
			return nil, std.NewLocalizedError("Foto nicht gefunden", "Das Foto ist nicht mehr vorhanden.")
		}

		reader := optReader.Unwrap()
		defer func() {
			_ = reader.Close()
		}()

		buf, err := Render(reader, tpl, raster())
		if err != nil {
			return nil, fmt.Errorf("cannot render preview: %w", err)
		}

		return buf, nil
	}
}
