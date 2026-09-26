package xgift

import (
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"
)

// Grid arranges children in rows of the given number of equally wide
// columns.
//
// gift has no grid container; a grid is rows of flexible cells. The last row
// is padded with empty cells so that its items keep the width of the ones
// above instead of stretching.
func Grid(columns int, gap float32, children ...gift.View) ui.Stack {
	if columns < 1 {
		columns = 1
	}

	rows := make([]gift.View, 0, (len(children)+columns-1)/columns)
	for i := 0; i < len(children); i += columns {
		cells := make([]gift.View, 0, columns)
		for j := range columns {
			if i+j < len(children) {
				cells = append(cells, Fill(children[i+j]).Flex(1))
			} else {
				cells = append(cells, ui.Box().Flex(1))
			}
		}

		rows = append(rows, ui.HStack(cells...).Gap(gap).Align(geom.TopLeading))
	}

	return ui.VStack(rows...).Gap(gap)
}

// VHairline is a one pixel vertical separator, for example between a sidebar
// and the content next to it.
func VHairline() gift.View {
	return ui.Box().Frame(1, geom.Unbounded()).Background(ui.ColorSeparator)
}

// Hairline is a one pixel separator in the theme's separator colour.
func Hairline() gift.View {
	return ui.Box().Frame(geom.Unbounded(), 1).Background(ui.ColorSeparator)
}
