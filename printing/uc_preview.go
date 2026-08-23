package printing

import (
	"fmt"

	"go.wdy.de/nago/auth"
	"go.wdy.de/nago/pkg/std"

	"github.com/torbenschinke/eventprint/photo"
)

// NewPreview erzeugt den [Preview] Anwendungsfall. Er rendert exakt dasselbe
// Bild wie der Druck-Worker, sodass die Vorschau verbindlich ist – inklusive
// des automatischen Bildausschnitts.
func NewPreview(openOriginal photo.OpenOriginal, renderOptions func() RenderOptions) Preview {
	renderOptions = orDefaultRenderOptions(renderOptions)

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

		buf, err := RenderWithOptions(reader, tpl, NativeRaster4x6, renderOptions())
		if err != nil {
			return nil, fmt.Errorf("cannot render preview: %w", err)
		}

		return buf, nil
	}
}
