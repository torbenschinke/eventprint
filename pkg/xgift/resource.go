package xgift

import (
	"github.com/worldiety/gift"
)

// Resource is a value that is loaded off the UI goroutine and shown when it
// arrives.
//
// A view obtains one with [UseResource], reads it with [Resource.Read] (which
// subscribes the calling scope) and starts or restarts the work with
// [Resource.Load]. Loads are numbered, and the answer of a load that a newer
// one has replaced is dropped when it arrives, so a quick sequence of
// requests never shows a stale result after a fresh one.
//
// The value is held by the resource and not by a gift.State, because a state
// must be comparable and most loaded values — slices, maps, structs with
// slices — are not. A small revision counter carries the change notification
// instead.
type Resource[T any] struct {
	rev     *gift.State[int]
	value   T
	err     error
	loading bool
	loaded  bool
	gen     int
	key     any
	keyed   bool
}

// UseResource returns the resource named key of the calling component
// instance, creating it on first use.
func UseResource[T any](ctx *gift.Context, key string) *Resource[T] {
	holder := ctx.State(key, (*Resource[T])(nil))
	r := holder.Get()
	if r == nil {
		r = &Resource[T]{rev: ctx.State(key+"#rev", 0)}
		// Storing the pointer is a write during build. It is the first and
		// only one for this slot, it happens before anything read the slot,
		// and it therefore invalidates nothing.
		holder.Set(r)
	}

	ctx.Read(r.rev)

	return r
}

// Load starts fn on a new goroutine. The previous value stays visible until
// the new one arrives.
func (r *Resource[T]) Load(fn func() (T, error)) {
	r.gen++
	gen := r.gen
	r.loading = true

	go func() {
		v, err := fn()
		Post(func() {
			if gen != r.gen {
				return
			}

			r.value, r.err, r.loading, r.loaded = v, err, false, true
			r.rev.Set(r.rev.Get() + 1)
		})
	}()
}

// LoadOnce starts fn unless the resource was already loaded or is loading.
func (r *Resource[T]) LoadOnce(fn func() (T, error)) {
	if r.loaded || r.loading {
		return
	}

	r.Load(fn)
}

// LoadKeyed starts fn whenever key differs from the key of the previous
// LoadKeyed call. Views call it on every build with a key made of whatever
// the value depends on — a filter, a revision counter, a clock divided down
// to the refresh interval — and the load happens exactly when one of those
// changes. key must be comparable.
func (r *Resource[T]) LoadKeyed(key any, fn func() (T, error)) {
	if r.keyed && r.key == key {
		return
	}

	r.key, r.keyed = key, true
	r.Load(fn)
}

// Set replaces the value directly, on the UI goroutine.
func (r *Resource[T]) Set(v T) {
	r.gen++
	r.value, r.err, r.loading, r.loaded = v, nil, false, true
	r.rev.Set(r.rev.Get() + 1)
}

// Invalidate marks the value as stale so that the next LoadOnce loads again.
func (r *Resource[T]) Invalidate() { r.loaded = false }

// Version counts the values that arrived. It changes exactly when Value
// does, which makes it a cheap key for derived state such as the contents of
// a gallery.
func (r *Resource[T]) Version() int { return r.rev.Get() }

// Value returns the last loaded value.
func (r *Resource[T]) Value() T { return r.value }

// Err returns the error of the last load.
func (r *Resource[T]) Err() error { return r.err }

// Loading reports whether a load is in flight.
func (r *Resource[T]) Loading() bool { return r.loading }

// Loaded reports whether a value has arrived at least once.
func (r *Resource[T]) Loaded() bool { return r.loaded }
