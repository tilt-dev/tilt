package hud

type Hud struct {
	Model *Model
	View  *View
}

func NewHud() *Hud {
	return &Hud{
		Model: NewModel(),
		View:  &View{},
	}
}

func (h *Hud) Render() {
	h.Model.Mu.Lock()
	defer h.Model.Mu.Unlock()

	h.View.Render(h.Model)
}
