package xgift

import (
	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
)

var fillType = gift.RegisterType("xgift.Fill")

// FillView takes all the space its parent offers and hands exactly that
// space to its child as tight constraints.
//
// gift's stacks and overlays measure their children with loose constraints,
// and a stack sizes itself to its content. That is the right default, but it
// means a vertically flexible child deep inside a stack never sees the height
// of the screen: the stack around it shrinks to its natural height first, and
// the flexible child gets nothing. The same happens across a gift.Component,
// which does not forward the Flex of the view it builds. Fill is the explicit
// way to say "this region is the whole remaining area": wrap the region, and
// everything inside it can use Flex along both axes again.
//
// Either axis can be fixed instead, for a sidebar of a given width that
// should still span the full height.
type FillView struct {
	child gift.View
	w, h  float32
	flex  float32
	key   string
}

// Fill wraps child.
func Fill(child gift.View) FillView { return FillView{child: child} }

// Width fixes the width; zero means "all that is offered".
func (f FillView) Width(v float32) FillView { f.w = v; return f }

// Height fixes the height; zero means "all that is offered".
func (f FillView) Height(v float32) FillView { f.h = v; return f }

// Flex sets the share of the remaining space along the main axis of an
// enclosing stack.
func (f FillView) Flex(v float32) FillView { f.flex = v; return f }

// Key sets the identity among siblings.
func (f FillView) Key(v string) FillView { f.key = v; return f }

// ViewType implements gift.View.
func (f FillView) ViewType() gift.TypeID { return fillType }

// Build implements gift.View.
func (f FillView) Build(*gift.BuildContext) gift.Element {
	return gift.Element{
		Key:      f.key,
		Flex:     f.flex,
		Layouter: fillLayout{w: f.w, h: f.h},
		Children: []gift.View{f.child},
	}
}

type fillLayout struct{ w, h float32 }

// Layout takes the maximum on every axis that is bounded and not fixed. An
// unbounded axis — inside a scroll view — falls back to the child's natural
// extent, because "all the space" is not a size there.
func (l fillLayout) Layout(ctx *gift.LayoutContext, c geom.Constraints) geom.Size {
	size := c.Max

	if l.w > 0 {
		size.W = c.ConstrainWidth(l.w)
	}

	if l.h > 0 {
		size.H = c.ConstrainHeight(l.h)
	}

	if ctx.ChildCount() == 0 {
		return c.Constrain(geom.Size{W: finite(size.W), H: finite(size.H)})
	}

	if !finiteSize(size) {
		natural := ctx.Measure(0, geom.Constraints{Max: size})
		if !isFinite(size.W) {
			size.W = natural.W
		}

		if !isFinite(size.H) {
			size.H = natural.H
		}
	}

	ctx.Measure(0, geom.Tight(size))
	ctx.Place(0, geom.Point{})

	return size
}

func isFinite(v float32) bool { return v < geom.Unbounded() }
func finite(v float32) float32 {
	if isFinite(v) {
		return v
	}
	return 0
}
func finiteSize(s geom.Size) bool { return isFinite(s.W) && isFinite(s.H) }
