package hud

import "github.com/windmilleng/tilt/internal/hud/model"

type Hud struct {
	Model    model.Model
	renderer Renderer
}

func (h *Hud) Refresh() error {
	return h.renderer.Render(h.Model)
}
