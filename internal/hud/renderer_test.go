package hud

import (
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"

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
	rtf.run("one undeployed resource", 70, 20, v)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                "a-a-a-aaaaabe vigoda",
				LastBuildFinishTime: time.Now(),
				LastBuildError:      "oh no the build failed",
				LastBuildLog:        "1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("inline build log", 70, 20, v)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:           "a-a-a-aaaaabe vigoda",
				LastBuildError: "oh no the build failed",
				LastBuildLog:   "1\n2\n3\nthe compiler wasn't smart enough to figure out what you meant!\n5\n6\n7\n8\n",
			},
		},
		ViewState: view.ViewState{
			LogModal: view.LogModal{ResourceLogNumber: 1},
		},
	}
	rtf.run("modal build log displayed", 70, 20, v)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:        "a-a-a-aaaaabe vigoda",
				PodName:     "vigoda-pod",
				PodStatus:   "Running",
				PodRestarts: 1,
				Endpoints:   []string{"1.2.3.4:8080"},
				PodLog:      "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("pod log displayed inline", 70, 20, v)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				LastManifestLoadError: "broken tiltfile!",
				LastBuildError:        "broken go code!",
				LastBuildLog:          "mashing keys is not a good way to generate code",
			},
		},
	}
	rtf.run("manifest error and build error", 70, 20, v)

	ts := time.Now().Add(-5 * time.Minute)
	v = view.View{
		Resources: []view.Resource{
			{
				Name:                  "a-a-a-aaaaabe vigoda",
				DirectoriesWatched:    []string{"foo", "bar"},
				LastDeployTime:        ts,
				LastDeployEdits:       []string{"main.go", "cli.go"},
				LastBuildError:        "the build failed!",
				LastBuildFinishTime:   ts,
				LastBuildDuration:     1400 * time.Millisecond,
				LastBuildLog:          "",
				PendingBuildEdits:     []string{"main.go", "cli.go", "vigoda.go"},
				PendingBuildSince:     ts,
				CurrentBuildEdits:     []string{"main.go"},
				CurrentBuildStartTime: ts,
				PodName:               "vigoda-pod",
				PodCreationTime:       ts,
				PodStatus:             "Running",
				PodRestarts:           1,
				Endpoints:             []string{"1.2.3.4:8080"},
				PodLog:                "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("all the data at once", 70, 20, v)
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
	t := time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC)
	r := NewRenderer(func() time.Time { return t })
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""))
	c := r.layout(v)
	rtf.i.Run(name, w, h, c)
}

var screen tcell.Screen

func TestMain(m *testing.M) {
	rty.InitScreenAndRun(m, &screen)
}
