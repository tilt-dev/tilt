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

	vs := view.ViewState{
		Resources: []view.ResourceViewState{
			{
				IsCollapsed: false,
			},
		},
	}

	rtf.run("one undeployed resource", 70, 20, v, vs)

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
	rtf.run("inline build log", 70, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                "a-a-a-aaaaabe vigoda",
				LastBuildFinishTime: time.Now(),
				LastBuildError:      "oh no the build failed",
				LastBuildLog: `STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
    ╎ RUNNING: go install github.com/windmilleng/servantes/snack

    ╎ ERROR IN: go install github.com/windmilleng/servantes/snack
    ╎   → # github.com/windmilleng/servantes/snack
src/github.com/windmilleng/servantes/snack/main.go:16:36: syntax error: unexpected newline, expecting comma or }

ERROR: ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code 2`,
			},
		},
	}
	rtf.run("inline build log with wrapping", 117, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:           "a-a-a-aaaaabe vigoda",
				LastBuildError: "oh no the build failed",
				LastBuildLog:   "1\n2\n3\nthe compiler wasn't smart enough to figure out what you meant!\n5\n6\n7\n8\n",
			},
		},
	}

	vs.LogModal = view.LogModal{ResourceLogNumber: 1}

	rtf.run("modal build log displayed", 70, 20, v, vs)

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
	vs.LogModal = view.LogModal{}
	rtf.run("pod log displayed inline", 70, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                  "a-a-a-aaaaabe vigoda",
				LastManifestLoadError: "broken tiltfile!",
				LastBuildError:        "broken go code!",
				LastBuildLog:          "mashing keys is not a good way to generate code",
			},
		},
	}
	rtf.run("manifest error and build error", 70, 20, v, vs)

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
	rtf.run("all the data at once", 70, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                  "GlobalYAML",
				CurrentBuildStartTime: ts,
				LastBuildFinishTime:   ts,
				LastBuildDuration:     1400 * time.Millisecond,
				LastDeployTime:        ts,
				LastBuildError:        "",
				IsYAMLManifest:        true,
			},
		},
	}
	rtf.run("global yaml manifest", 70, 20, v, vs)

	vs.AlertMessage = "this is only a test"
	rtf.run("alert message", 70, 20, v, vs)
}

func TestRenderNarrationMessage(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{}
	vs := view.ViewState{
		ShowNarration:    true,
		NarrationMessage: "hi mom",
	}

	rtf.run("narration message", 60, 20, v, vs)
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

func (rtf rendererTestFixture) run(name string, w int, h int, v view.View, vs view.ViewState) {
	t := time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC)
	r := NewRenderer(func() time.Time { return t })
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""))
	c := r.layout(v, vs)
	rtf.i.Run(name, w, h, c)
}

var screen tcell.Screen

func TestMain(m *testing.M) {
	rty.InitScreenAndRun(m, &screen)
}
