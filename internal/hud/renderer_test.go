package hud

import (
	"testing"

	"github.com/windmilleng/tilt/internal/rty"

	"github.com/windmilleng/tilt/internal/hud/view"

	"github.com/windmilleng/tcell"
)

func TestRender(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:               "foo",
				DirectoriesWatched: []string{"bar"},
			},
		},
	}

	rtf.run("one undeployed resource", 60, 20, v)
}

func TestRenderNarrationMessage(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		ViewState: view.ViewState{
			ShowNarration:    true,
			NarrationMessage: "hi mom",
		},
	}

	rtf.run("narration message", 60, 20, v)
}

type rendererTestFixture struct {
	t *testing.T
	i rty.InteractiveTester
}

func newRendererTestFixture(t *testing.T) rendererTestFixture {
	return rendererTestFixture{
		t: t,
		i: rty.NewInteractiveTester(t, screen),
	}
}

func (rtf rendererTestFixture) run(name string, w int, h int, v view.View) {
	r := NewRenderer()
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""))
	c := r.layout(v)
	rtf.i.Run(name, w, h, c)
}

var screen tcell.Screen

func TestMain(m *testing.M) {
	rty.InitScreenAndRun(m, &screen)
}
