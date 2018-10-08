package hud

import (
	"context"

	"github.com/windmilleng/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	LastView view.View
}

func (h *FakeHud) Run(ctx context.Context) error { return nil }

func (h *FakeHud) Update(v view.View) {
	h.LastView = v
}
