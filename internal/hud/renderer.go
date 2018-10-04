package hud

import (
	"context"

	"github.com/windmilleng/tilt/internal/hud/view"
)

type Renderer struct {
	ttyPath string
	ctx     context.Context
}

func (r *Renderer) Render(v view.View) error {
	// TODO: draw v
	// if r.ttyPath != "" {
	//     log.Infof("drawing to tty: %s\n", r.ttyPath)
	// }
	return nil
}

func (r *Renderer) Reset() {
	r.ttyPath = ""
	r.ctx = nil
}
