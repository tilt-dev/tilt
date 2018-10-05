package hud

import (
	"context"

	"log"

	"github.com/windmilleng/tilt/internal/hud/view"
)

type HeadsUpDisplay interface {
	Run(ctx context.Context)
	Update(v view.View)
}

type Hud struct {
	a *ServerAdapter
	r *Renderer
}

var _ HeadsUpDisplay = (*Hud)(nil)

func NewDefaultHeadsUpDisplay() (HeadsUpDisplay, error) {
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
	err := h.r.Render(v)
	if err != nil {
		log.Println("Error rendering HUD")
	}
}
