package upld

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"go.wdy.de/nago/application/image"
	"go.wdy.de/nago/application/user"
	"go.wdy.de/nago/pkg/blob"
)

const ImagePrefix = "photoupld-"

// Purger removes an original, all pyramid elements and the SrcSet descriptor.
type Purger struct {
	load  image.LoadSrcSet
	sets  blob.Store
	blobs blob.Store
}

func NewPurger(load image.LoadSrcSet, sets, blobs blob.Store) Purger {
	return Purger{load: load, sets: sets, blobs: blobs}
}

func (p Purger) Delete(id image.ID) {
	set, err := p.load(user.SU(), id)
	if err != nil {
		slog.Error("cannot load upload image for purge", "image", id, "err", err)
		return
	}
	if set.IsSome() {
		for _, level := range set.Unwrap().Images {
			if err := p.blobs.Delete(context.Background(), string(level.Data)); err != nil {
				slog.Error("cannot purge upload image level", "image", level.Data, "err", err)
			}
		}
	}
	if err := p.sets.Delete(context.Background(), string(id)); err != nil {
		slog.Error("cannot purge upload srcset", "image", id, "err", err)
	}
}

// DeleteOrphans removes relay images left behind by a process restart.
func (p Purger) DeleteOrphans() error {
	var ids []image.ID
	for key, err := range p.sets.List(context.Background(), blob.ListOptions{Prefix: ImagePrefix}) {
		if err != nil {
			return fmt.Errorf("cannot list upload srcsets: %w", err)
		}
		if strings.HasPrefix(key, ImagePrefix) {
			ids = append(ids, image.ID(key))
		}
	}
	for _, id := range ids {
		p.Delete(id)
	}
	return nil
}
