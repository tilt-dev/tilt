package hud

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	LastView view.View
	Updates  chan view.View
	Canceled bool
}

func NewFakeHud() *FakeHud {
	return &FakeHud{
		Updates: make(chan view.View, 10),
	}
}

func (h *FakeHud) Run(ctx context.Context, st *store.Store) error {
	<-ctx.Done()
	h.Canceled = true
	return ctx.Err()
}

func (h *FakeHud) OnChange(ctx context.Context, st *store.Store) {
	onChange(ctx, st, h)
}

func (h *FakeHud) Update(v view.View) error {
	h.LastView = v
	h.Updates <- v
	return nil
}
