package lightroom

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Assets listet eine Seite Fotos eines Albums; ein leeres Album steht für alle
// Fotos des Kontos.
//
// Seitenweise, weil "alle Fotos" leicht zehntausende sind. cursor ist leer für
// die erste Seite und sonst [Page.Next] der vorigen.
type Assets func(subject permission.Auditable, ctx context.Context, albumID string, cursor string) (Page, error)

// pageSize ist die Zahl der Fotos je Anfrage an Lightroom.
const pageSize = "100"

// maxEmptyPages begrenzt das Weiterblättern über Seiten, die nur Videos
// enthalten. Eine leere Seite mit Folgemarke sähe auf dem Touchscreen aus wie
// ein leeres Album; einige Seiten weiterzublättern erspart das, ohne bei einem
// reinen Videoalbum endlos zu laden.
const maxEmptyPages = 10

// NewAssets bindet die Fotoliste an den Client.
func NewAssets(client *Client) Assets {
	return func(subject permission.Auditable, ctx context.Context, albumID string, cursor string) (Page, error) {
		if err := subject.Audit(PermAssets); err != nil {
			return Page{}, err
		}

		cfg := client.cfg()

		cid, err := client.catalog(ctx, cfg)
		if err != nil {
			return Page{}, err
		}

		albumID = strings.TrimSpace(albumID)
		embedded := albumID != ""

		var target string

		switch {
		case cursor != "":
			// Die Marke kommt aus der Oberfläche zurück. Sie muss in den
			// eigenen Katalog zeigen; alles andere ist keine Marke, die diese
			// Box ausgegeben hat.
			if !strings.Contains(cursor, "/catalogs/"+cid+"/") {
				return Page{}, errors.New("ungültige Seitenmarke")
			}

			if target, err = cursorURL(cfg, cursor); err != nil {
				return Page{}, err
			}
		case embedded:
			target = cfg.APIBaseURL + "/v2/catalogs/" + url.PathEscape(cid) + "/albums/" + url.PathEscape(albumID) +
				"/assets?embed=asset&limit=" + pageSize
		default:
			target = cfg.APIBaseURL + "/v2/catalogs/" + url.PathEscape(cid) + "/assets?subtype=image&limit=" + pageSize
		}

		page := Page{Assets: []Asset{}}

		for range maxEmptyPages {
			var out struct {
				listEnvelope
				Resources []json.RawMessage `json:"resources"`
			}

			if err := client.getJSON(ctx, cfg, target, &out); err != nil {
				return Page{}, err
			}

			page.Assets = append(page.Assets, parseAssets(out.Resources)...)

			next, err := nextCursor(cfg, target, out.listEnvelope)
			if err != nil {
				return Page{}, err
			}

			page.Next = next
			if next == "" || len(page.Assets) > 0 {
				break
			}

			if target, err = cursorURL(cfg, next); err != nil {
				return Page{}, err
			}
		}

		return page, nil
	}
}

// parseAssets liest Fotos aus beiden Listenformen.
//
// In einem Album steckt das Foto unter "asset" (mit embed=asset), in der Liste
// aller Fotos ist die Ressource selbst das Foto. Beide Formen zu lesen erspart
// es, sich die Herkunft der Seite über die Marke hinweg zu merken.
func parseAssets(raw []json.RawMessage) []Asset {
	type resource struct {
		assetJSON
		Asset *assetJSON `json:"asset"`
	}

	var out []Asset

	for _, r := range decodeResources[resource](raw) {
		a := r.assetJSON
		if r.Asset != nil {
			a = *r.Asset
			if a.ID == "" {
				a.ID = r.ID
			}
		}

		if a.ID == "" || !a.isImage() {
			continue
		}

		out = append(out, a.toAsset())
	}

	return out
}
