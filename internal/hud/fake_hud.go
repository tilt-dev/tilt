package hud

import (
	"context"

	"github.com/windmilleng/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	LastView view.View
	Updates  chan view.View
}

func NewFakeHud() *FakeHud {
	return &FakeHud{
		Updates: make(chan view.View, 10),
	}
}

func (h *FakeHud) Run(ctx context.Context) {}

func (h *FakeHud) Update(v view.View) {
	h.LastView = v
	h.Updates <- v
}
