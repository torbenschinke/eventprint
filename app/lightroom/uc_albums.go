package lightroom

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Albums listet die Alben des verbundenen Kontos, nach Namen sortiert.
//
// Nur Alben, keine Ordner ("collection_set"): Ein Ordner enthält selbst keine
// Fotos, und auf dem Touchscreen ist eine flache Liste schneller zu bedienen
// als ein Baum.
type Albums func(subject permission.Auditable, ctx context.Context) ([]Album, error)

// maxPages begrenzt das Blättern. Eine Gegenstelle, die immer wieder auf
// dieselbe oder eine weitere Seite verweist, soll die Box nicht endlos
// beschäftigen.
const maxPages = 200

// NewAlbums bindet die Albenliste an den Client.
//
// Die Alben kommen vollständig und nicht seitenweise, weil ein Konto selten
// mehr als einige hundert davon hat und die Oberfläche sie sortiert zeigen
// will – das geht nur mit allen.
func NewAlbums(client *Client) Albums {
	return func(subject permission.Auditable, ctx context.Context) ([]Album, error) {
		if err := subject.Audit(PermAlbums); err != nil {
			return nil, err
		}

		cfg := client.cfg()

		cid, err := client.catalog(ctx, cfg)
		if err != nil {
			return nil, err
		}

		target := cfg.APIBaseURL + "/v2/catalogs/" + url.PathEscape(cid) + "/albums?subtype=collection"
		seen := map[string]bool{}

		var albums []Album

		for page := 0; target != ""; page++ {
			if page >= maxPages || seen[target] {
				return nil, errors.New("Lightroom liefert eine endlose Albenliste")
			}

			seen[target] = true

			var out struct {
				listEnvelope
				Resources []json.RawMessage `json:"resources"`
			}

			if err := client.getJSON(ctx, cfg, target, &out); err != nil {
				return nil, err
			}

			type albumJSON struct {
				ID      string `json:"id"`
				Subtype string `json:"subtype"`
				Payload struct {
					Name  string `json:"name"`
					Cover *struct {
						ID string `json:"id"`
					} `json:"cover"`
				} `json:"payload"`
			}

			for _, a := range decodeResources[albumJSON](out.Resources) {
				if a.ID == "" || (a.Subtype != "" && a.Subtype != "collection") {
					continue
				}

				album := Album{ID: a.ID, Name: strings.TrimSpace(a.Payload.Name)}
				if a.Payload.Cover != nil {
					album.CoverAssetID = a.Payload.Cover.ID
				}

				albums = append(albums, album)
			}

			next, err := nextCursor(cfg, target, out.listEnvelope)
			if err != nil {
				return nil, err
			}

			target = ""
			if next != "" {
				if target, err = cursorURL(cfg, next); err != nil {
					return nil, err
				}
			}
		}

		sort.SliceStable(albums, func(i, j int) bool {
			return strings.ToLower(albums[i].Name) < strings.ToLower(albums[j].Name)
		})

		return albums, nil
	}
}
