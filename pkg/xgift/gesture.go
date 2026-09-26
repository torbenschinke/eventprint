package xgift

import "time"

// TapGate opens after a number of quick taps, as a hidden door on a kiosk.
//
// Only consecutive taps within the window count, so that the occasional
// curious tap of passers-by over an evening does not add up to an opening.
// The zero value is not usable; use [NewTapGate].
type TapGate struct {
	needed int
	window time.Duration
	count  int
	last   time.Time
	now    func() time.Time
}

// NewTapGate returns a gate that opens after needed taps with at most window
// between two of them.
func NewTapGate(needed int, window time.Duration) *TapGate {
	return &TapGate{needed: needed, window: window, now: time.Now}
}

// Tap counts one tap and reports whether the gate opens. On success the
// count starts over.
func (g *TapGate) Tap() bool {
	now := g.now()
	if g.count > 0 && now.Sub(g.last) > g.window {
		g.count = 0
	}

	g.count++
	g.last = now

	if g.count >= g.needed {
		g.count = 0
		return true
	}

	return false
}
