package xgift

import (
	"time"

	"github.com/worldiety/gift"
	"github.com/worldiety/gift/geom"
	"github.com/worldiety/gift/ui"
)

var slideType = gift.RegisterType("xgift.Slide")

// SlideView is a full-size layer whose change of visibility is a movement.
//
// It is the building block of [Pages] and carries a [gift.TransitionSpec]:
// while hidden it waits at Parked, a freshly mounted visible slide comes in
// from Entry, and one that is shown again comes back from Return. All offsets
// are fractions of its own size.
type SlideView struct {
	key                string
	child              gift.View
	hidden             bool
	parked, entry, ret geom.Point
	duration           time.Duration
}

// Slide wraps child. key identifies the slide among its siblings, and it is
// the key that lets the same screen keep its state while it moves.
func Slide(key string, child gift.View) SlideView {
	return SlideView{key: key, child: child, duration: PageAnimation}
}

// Hidden parks the slide.
func (s SlideView) Hidden(v bool) SlideView { s.hidden = v; return s }

// Parked is where the slide waits while hidden.
func (s SlideView) Parked(p geom.Point) SlideView { s.parked = p; return s }

// Entry is where a slide mounted visible comes from; zero means Parked.
func (s SlideView) Entry(p geom.Point) SlideView { s.entry = p; return s }

// Return is where a slide shown again comes from; zero means from where it
// went.
func (s SlideView) Return(p geom.Point) SlideView { s.ret = p; return s }

// Duration sets how long the movement takes; zero cuts.
func (s SlideView) Duration(d time.Duration) SlideView { s.duration = d; return s }

// ViewType implements gift.View.
func (s SlideView) ViewType() gift.TypeID { return slideType }

// Build implements gift.View.
func (s SlideView) Build(*gift.BuildContext) gift.Element {
	return gift.Element{
		Key:      s.key,
		Layouter: fillLayout{},
		Children: []gift.View{s.child},
		Hidden:   s.hidden,
		Transition: gift.TransitionSpec{
			Parked:   s.parked,
			Entry:    s.entry,
			Return:   s.ret,
			Duration: s.duration,
		},
	}
}

// PageAnimation is the length of a page movement: long enough to follow
// where a screen went, short enough not to wait for it.
const PageAnimation = 280 * time.Millisecond

// Page is one screen of a hierarchy for [Pages].
type Page struct {
	// Key identifies the screen. The same key is the same screen, with its
	// state, scroll offset and half typed text.
	Key string

	// Depth orders screens: a deeper screen is pushed from the trailing edge
	// over a shallower one, and going back to a shallower one pops it off
	// again. Screens of the same depth replace each other like a push.
	Depth int

	View gift.View
}

// underlap is how far a covered screen moves towards the leading edge while
// the next one slides over it, as a fraction of its width. It is the depth
// cue of iOS: the screen underneath does not stand still and does not leave
// either.
const underlap = 0.3

// Pages shows current and animates the way from previous to it: forward
// slides current in from the trailing edge over previous, back slides
// previous out to the trailing edge and uncovers current.
//
// Unlike ui.NavigationStack, which unmounts a popped screen and can therefore
// only animate the half that remains, both screens stay mounted for the
// length of the movement — the caller keeps passing the previous page until
// the next navigation replaces it. That is the whole state this needs: the
// application owns "where am I" and "where did I come from", and Pages owns
// nothing.
//
// Every page must paint an opaque background, because the two overlap while
// they move.
func Pages(current, previous Page) gift.View {
	if previous.View == nil || previous.Key == current.Key {
		return ui.ZStack(Slide(current.Key, current.View)).Flex(1)
	}

	forward := current.Depth >= previous.Depth

	cur := Slide(current.Key, current.View)
	prev := Slide(previous.Key, previous.View).Hidden(true)

	if forward {
		cur = cur.Entry(geom.Pt(1, 0)).Parked(geom.Pt(1, 0))
		prev = prev.Parked(geom.Pt(-underlap, 0))

		return ui.ZStack(prev, cur).Flex(1)
	}

	cur = cur.Entry(geom.Pt(-underlap, 0)).Parked(geom.Pt(-underlap, 0))
	prev = prev.Parked(geom.Pt(1, 0))

	return ui.ZStack(cur, prev).Flex(1)
}
