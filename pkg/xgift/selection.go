package xgift

import (
	"github.com/worldiety/gift/asset"
	"github.com/worldiety/gift/ui"
)

// TouchSelect turns a gallery tap into a toggle.
//
// gift's gallery follows desktop conventions: a click selects only the
// clicked item, and toggling needs a modifier key that a touchscreen does not
// have. Call this from the gallery's OnSelect with the set the application
// keeps; it toggles id in that set and makes the gallery show exactly the
// set again. It returns the new set.
func TouchSelect(g *ui.Gallery, selected []asset.ID, id asset.ID) []asset.ID {
	out := make([]asset.ID, 0, len(selected)+1)
	found := false
	for _, s := range selected {
		if s == id {
			found = true
			continue
		}

		out = append(out, s)
	}

	if !found {
		out = append(out, id)
	}

	ShowSelection(g, out)

	return out
}

// ShowSelection makes the gallery's visible selection equal ids.
func ShowSelection(g *ui.Gallery, ids []asset.ID) {
	sel := g.Selection()
	sel.Clear()
	for _, id := range ids {
		sel.Set(id, true)
	}
}
