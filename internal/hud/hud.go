package hud

import (
	"context"

	"github.com/windmilleng/tilt/internal/hud/view"
)

type Hud struct {
	a *ServerAdapter
	r *Renderer
}

func NewHud() (*Hud, error) {
	a, err := NewServer()
	if err != nil {
		return nil, err
	}

	return &Hud{
		a: a,
		r: &Renderer{},
	}, nil
}

func (h *Hud) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ready := <-h.a.readyCh:
			h.r.ttyPath = ready.ttyPath
			h.r.ctx = ready.ctx
		case <-h.a.streamClosedCh:
			h.r.Reset()
		}

	}
}

func (h *Hud) Update(v view.View) {
	h.r.Render(v)
}
