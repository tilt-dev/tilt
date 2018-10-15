package hud

import (
	"context"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpDisplay interface {
	Run(ctx context.Context, st *store.Store) error
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

func (h *Hud) Run(ctx context.Context, st *store.Store) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ready := <-h.a.readyCh:
			err := h.r.SetUp(ready, st)
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
