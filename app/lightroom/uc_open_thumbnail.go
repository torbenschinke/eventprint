package lightroom

import (
	"context"
	"io"

	"go.wdy.de/nago/application/permission"
)

// OpenThumbnail öffnet das Vorschaubild eines Fotos für die Übersicht.
//
// Ein Datenstrom statt eines Puffers: Die Oberfläche reicht das Bild meist
// direkt an den Browser weiter, und bei hundert Vorschauen auf einer Seite
// zählt jeder Kopiervorgang auf dem Pi. Der Aufrufer muss schließen.
type OpenThumbnail func(subject permission.Auditable, ctx context.Context, assetID string) (io.ReadCloser, error)

// thumbnailRendition ist die kleine Stufe, die Lightroom für jedes Foto
// bereithält; scharf genug für ein Raster auf dem Touchscreen.
const thumbnailRendition = "thumbnail2x"

// NewOpenThumbnail bindet die Vorschau an den Client.
func NewOpenThumbnail(client *Client) OpenThumbnail {
	return func(subject permission.Auditable, ctx context.Context, assetID string) (io.ReadCloser, error) {
		if err := subject.Audit(PermOpenThumbnail); err != nil {
			return nil, err
		}

		return client.openRendition(ctx, client.cfg(), assetID, thumbnailRendition)
	}
}
