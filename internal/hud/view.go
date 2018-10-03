package hud

import "log"

// the View is long-lived. Eventually, it should have other attributes.
type View struct{}

func (v *View) Render(m *Model) {
	r := &Render{}
	r.Render(m)
}

// a Render is short-lived (for one render loop), and holds mutable state as we
// render down the screen. (It's a helper for the View.)
type Render struct{}

// Render renders the Model
func (r *Render) Render(m *Model) {
	log.Printf("rendering some info: %s", m.Info)
}
