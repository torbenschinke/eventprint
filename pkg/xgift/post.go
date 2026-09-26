package xgift

import (
	"sync/atomic"

	"github.com/worldiety/gift"
)

var app atomic.Pointer[gift.App]

// Install registers the application whose UI executor receives the results
// of asynchronous work started through this package.
//
// It is a package level handle for the same reason gift's own examples keep
// one: a view function deep in the tree needs to post back to the UI
// goroutine, and threading the *gift.App through every builder would be
// noise. Call it once, before the first frame.
func Install(a *gift.App) { app.Store(a) }

// Post hands fn to the UI executor. It may be called from any goroutine.
// Without an installed App fn is dropped: there is nobody left to show the
// result to.
func Post(fn func()) {
	if a := app.Load(); a != nil {
		a.Post(fn)
	}
}
