package hud

import (
	"context"
	"fmt"

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
	fmt.Println("running the hud!")
	// handle/return errs?
	for {
		select {
		case <-ctx.Done():
			fmt.Println("it's done!")
			return
		case ready := <-h.a.readyCh:
			fmt.Printf("got a ready!")
			h.r.ttyPath = ready.ttyPath
			h.r.ctx = ready.ctx
		case <-h.a.streamClosedCh:
			fmt.Printf("stream is closed, clearing out tty")
			h.r.Reset()
		}

	}
}

func (h *Hud) Update(v view.View) {
	h.r.Render(v)
}
