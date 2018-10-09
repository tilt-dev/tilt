package hud

import (
	"context"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/hud/view"
)

type HeadsUpDisplay interface {
	Run(ctx context.Context) error
	Update(v view.View) error
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
		r: NewRenderer(),
	}, nil
}

func (h *Hud) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ready := <-h.a.readyCh:
			err := h.r.SetUp(ready)
			if err != nil {
				return err
			}
		case <-h.a.streamClosedCh:
			h.r.Reset()
		}

	}
}

func (h *Hud) Update(v view.View) error {
	err := h.r.Render(v)
	return errors.Wrap(err, "error rendering hud")
}
