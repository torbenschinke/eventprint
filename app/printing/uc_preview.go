package printing

import (
	"fmt"

	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/app/photo"
)

// PreviewCmd beschreibt eine Vorschau.
type PreviewCmd struct {
	// Photos sind die Motive des Blattes; leer zeigt das Layout mit
	// Platzhaltern.
	Photos []photo.ID

	Layout Layout

	// MaxEdge ist die lange Kante der Vorschau in Pixeln.
	MaxEdge int
}

// Preview zeigt, wie ein Blatt gedruckt wird, als JPEG.
//
// Sie entsteht mit demselben Renderer wie der Ausdruck. Die Wahl des Layouts
// soll keine Überraschung sein, und das gelingt nur, wenn Vorschau und Druck
// nicht zwei getrennte Darstellungen sind.
type Preview func(subject permission.Auditable, cmd PreviewCmd) ([]byte, error)

// NewPreview erzeugt den [Preview] Anwendungsfall.
func NewPreview(motifs loadMotifs, renderOptions func() RenderOptions, visible func(permission.Auditable, []photo.ID) error) Preview {
	renderOptions = orDefaultRenderOptions(renderOptions)

	return func(subject permission.Auditable, cmd PreviewCmd) ([]byte, error) {
		if err := subject.Audit(PermPreview); err != nil {
			return nil, err
		}

		layout := cmd.Layout.Normalized()

		sheet := placeholderSheet(layout)
		if len(cmd.Photos) > 0 {
			if err := visible(subject, cmd.Photos); err != nil {
				return nil, err
			}

			var err error
			if sheet, err = motifs(cmd.Photos, layout); err != nil {
				return nil, err
			}
		}

		buf, err := PreviewSheet(sheet, cmd.MaxEdge, renderOptions())
		if err != nil {
			return nil, fmt.Errorf("cannot render preview: %w", err)
		}

		return buf, nil
	}
}
