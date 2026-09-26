package xgift

import (
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
)

var stretchType = gift.RegisterType("xgift.HStretch")

// HStretchView is a horizontal row whose children all span its full height.
//
// ui.HStack measures its children with an unbounded height and then sizes
// itself to the tallest one, which is right for a toolbar and wrong for a
// split view: a sidebar next to a scrolling detail should both be as tall as
// the screen, and a vertically flexible child needs a bounded height to flex
// into. HStretch gives every child the row's height as a tight constraint.
//
// Children with a Flex share the width that is left after the inflexible
// ones took their natural width, proportionally to their Flex, exactly like
// a stack's main axis.
type HStretchView struct {
	children []gift.View
	gap      float32
	flex     float32
	key      string
}

// HStretch returns a row of children spanning its full height.
func HStretch(children ...gift.View) HStretchView { return HStretchView{children: children} }

// Gap sets the space between two children.
func (v HStretchView) Gap(g float32) HStretchView { v.gap = g; return v }

// Flex sets the share along the main axis of an enclosing stack.
func (v HStretchView) Flex(f float32) HStretchView { v.flex = f; return v }

// Key sets the identity among siblings.
func (v HStretchView) Key(k string) HStretchView { v.key = k; return v }

// ViewType implements gift.View.
func (v HStretchView) ViewType() gift.TypeID { return stretchType }

// Build implements gift.View.
func (v HStretchView) Build(*gift.BuildContext) gift.Element {
	return gift.Element{Key: v.key, Flex: v.flex, Layouter: stretchLayout{gap: v.gap}, Children: v.children}
}

type stretchLayout struct{ gap float32 }

func (l stretchLayout) Layout(ctx *gift.LayoutContext, c geom.Constraints) geom.Size {
	n := ctx.ChildCount()
	if n == 0 {
		return c.Constrain(geom.Size{})
	}

	width := c.Max.W
	if !isFinite(width) {
		width = 0
	}

	height := c.Max.H
	bounded := isFinite(height)

	gaps := l.gap * float32(n-1)
	used := gaps
	var flexSum float32
	sizes := make([]geom.Size, n)

	// Inflexible children first, with the full height and whatever width is
	// left. Without a bounded height they size themselves and the row
	// becomes as tall as the tallest of them.
	for i := range n {
		f := ctx.ChildFlex(i)
		if f > 0 {
			flexSum += f
			continue
		}

		cc := geom.Constraints{Max: geom.Size{W: max(0, width-used), H: height}}
		if bounded {
			cc.Min.H = height
		}

		sizes[i] = ctx.Measure(i, cc)
		used += sizes[i].W
	}

	if !bounded {
		height = 0
		for _, s := range sizes {
			height = max(height, s.H)
		}
	}

	rest := max(0, width-used)
	for i := range n {
		f := ctx.ChildFlex(i)
		if f <= 0 {
			continue
		}

		w := rest * f / flexSum
		sizes[i] = ctx.Measure(i, geom.Tight(geom.Size{W: w, H: height}))
	}

	x := float32(0)
	for i := range n {
		ctx.Place(i, geom.Point{X: x})
		x += sizes[i].W + l.gap
	}

	total := x - l.gap
	if flexSum > 0 && isFinite(c.Max.W) {
		total = c.Max.W
	}

	return c.Constrain(geom.Size{W: total, H: height})
}
