package lightroom

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.wdy.de/nago/application/permission"
)

// Download lädt ein Foto in Druckqualität auf die Box.
type Download func(subject permission.Auditable, ctx context.Context, assetID string) (Photo, error)

// Die Stufen, in dieser Reihenfolge versucht.
//
// 2048 Pixel an der langen Kante reichen für 10x15 cm bei 300 dpi, die 1800 x
// 1200 Pixel verlangen. Das Original wäre größer, müsste aber erst bei Adobe
// erzeugt werden und kann ein RAW sein, das der Drucker nicht versteht. Die
// 2048er-Stufe fehlt bei manchen Fotos, solange Lightroom sie noch nicht
// berechnet hat; dann ist 1280 besser als nichts – ein etwas weicher Druck
// statt keines.
//
// Konstanten und keine Liste als Paketvariable: Der Anwendungsfall soll keinen
// veränderlichen Paketzustand lesen.
const (
	printRendition    = "2048"
	fallbackRendition = "1280"
)

// NewDownload bindet das Herunterladen an den Client.
func NewDownload(client *Client) Download {
	return func(subject permission.Auditable, ctx context.Context, assetID string) (Photo, error) {
		if err := subject.Audit(PermDownload); err != nil {
			return Photo{}, err
		}

		cfg := client.cfg()

		data, err := downloadRendition(ctx, client, cfg, assetID)
		if err != nil {
			return Photo{}, err
		}

		return Photo{Name: archiveName(fileNameOf(ctx, client, cfg, assetID), assetID), Data: data}, nil
	}
}

func downloadRendition(ctx context.Context, client *Client, cfg Config, assetID string) ([]byte, error) {
	var lastErr error

	for _, kind := range [...]string{printRendition, fallbackRendition} {
		rc, err := client.openRendition(ctx, cfg, assetID, kind)
		if isStatus(err, http.StatusNotFound) {
			lastErr = err
			continue
		}

		if err != nil {
			return nil, err
		}

		// Die Grenze sitzt schon im Datenstrom; ein Byte darüber hinaus zu
		// verlangen zeigt, ob sie gegriffen hat, statt ein abgeschnittenes
		// JPEG stillschweigend zu drucken.
		data, err := io.ReadAll(io.LimitReader(rc, maxRendition+1))
		_ = rc.Close()

		if err != nil {
			return nil, fmt.Errorf("Foto aus Lightroom bricht beim Laden ab: %w", err)
		}

		if len(data) >= maxRendition {
			return nil, errors.New("Foto aus Lightroom ist unerwartet groß")
		}

		if len(data) == 0 {
			return nil, errors.New("Lightroom liefert ein leeres Foto")
		}

		return data, nil
	}

	return nil, fmt.Errorf("für dieses Foto liefert Lightroom kein Bild in Druckqualität: %w", lastErr)
}

// fileNameOf holt den Originalnamen des Fotos.
//
// Scheitert das, wird trotzdem gedruckt: Der Name dient nur dem Archiv, und
// die Box erfindet dann einen aus der Kennung.
func fileNameOf(ctx context.Context, client *Client, cfg Config, assetID string) string {
	cid, err := client.catalog(ctx, cfg)
	if err != nil {
		return ""
	}

	var a assetJSON
	if err := client.getJSON(ctx, cfg, "/v2/catalogs/"+url.PathEscape(cid)+"/assets/"+url.PathEscape(assetID), &a); err != nil {
		return ""
	}

	return a.Payload.ImportSource.FileName
}
