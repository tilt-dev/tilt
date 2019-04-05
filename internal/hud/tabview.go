package hud

import (
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

type TabView struct {
	view      view.View
	viewState view.ViewState
}

func NewTabView(v view.View, vState view.ViewState) *TabView {
	return &TabView{
		view:      v,
		viewState: vState,
	}
}

func (v *TabView) Build() rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	log := rty.NewTextScrollLayout("log")
	log.Add(rty.TextString(v.view.Log.String()))
	l.Add(log)
	return l
}
