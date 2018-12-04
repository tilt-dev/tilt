package hud

import (
	"strings"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
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

	plainVs := view.ViewState{
		Resources: []view.ResourceViewState{
			{},
		},
	}

	rtf.run("one undeployed resource", 70, 20, v, plainVs)

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
	rtf.run("inline build log", 70, 20, v, plainVs)

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
	rtf.run("inline build log with wrapping", 117, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:           "a-a-a-aaaaabe vigoda",
				LastBuildError: "oh no the build failed",
				LastBuildLog:   "1\n2\n3\nthe compiler wasn't smart enough to figure out what you meant!\n5\n6\n7\n8\n",
			},
		},
	}

	logModalVs := plainVs
	logModalVs.LogModal = view.LogModal{ResourceLogNumber: 1}

	rtf.run("modal build log displayed", 70, 20, v, logModalVs)

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

	rtf.run("pod log displayed inline", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:           "a-a-a-aaaaabe vigoda",
				LastBuildError: "broken go code!",
				LastBuildLog:   "mashing keys is not a good way to generate code",
			},
		},
	}
	rtf.run("manifest error and build error", 70, 20, v, plainVs)

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
	rtf.run("all the data at once", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                  "abe vigoda",
				DirectoriesWatched:    []string{"foo", "bar"},
				LastDeployTime:        ts,
				LastDeployEdits:       []string{"main.go"},
				PendingBuildEdits:     []string{},
				PendingBuildSince:     ts,
				CurrentBuildEdits:     []string{},
				CurrentBuildStartTime: ts,
				CurrentBuildReason:    model.BuildReasonFlagCrash,
				PodName:               "vigoda-pod",
				PodCreationTime:       ts,
				PodStatus:             "Running",
				PodRestarts:           0,
				Endpoints:             []string{"1.2.3.4:8080"},
				PodLog:                "",
				CrashLog:              "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("crash rebuild", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                "vigoda",
				DirectoriesWatched:  []string{"foo", "bar"},
				LastDeployTime:      ts,
				LastDeployEdits:     []string{"main.go", "cli.go"},
				LastBuildFinishTime: ts,
				LastBuildDuration:   1400 * time.Millisecond,
				LastBuildLog:        "",
				PodName:             "vigoda-pod",
				PodCreationTime:     ts,
				PodStatus:           "Running",
				PodRestarts:         1,
				Endpoints:           []string{"1.2.3.4:8080"},
				PodLog: `abe vigoda is crashing
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo`,
			},
		},
	}
	rtf.run("pod log with inline wrapping", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                "GlobalYAML",
				LastBuildFinishTime: ts,
				LastBuildDuration:   1400 * time.Millisecond,
				LastDeployTime:      ts,
				LastBuildError:      "",
				IsYAMLManifest:      true,
			},
		},
	}
	rtf.run("global yaml manifest", 70, 20, v, plainVs)

	alertVs := plainVs
	alertVs.AlertMessage = "this is only a test"
	rtf.run("alert message", 70, 20, v, alertVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:                  "vigoda",
				CurrentBuildStartTime: ts.Add(-5 * time.Second),
				CurrentBuildEdits:     []string{"main.go"},
			},
		},
	}
	rtf.run("build in progress", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:              "vigoda",
				PendingBuildSince: ts.Add(-5 * time.Second),
				PendingBuildEdits: []string{"main.go"},
			},
		},
	}
	rtf.run("pending build", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:            "vigoda",
				LastDeployTime:  ts.Add(-5 * time.Second),
				LastDeployEdits: []string{"abbot.go", "costello.go", "harold.go"},
			},
		},
	}
	rtf.run("edited files narrow term", 60, 20, v, plainVs)
	rtf.run("edited files normal term", 80, 20, v, plainVs)
	rtf.run("edited files wide term", 120, 20, v, plainVs)
}

func TestRenderTiltLog(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Log:       strings.Repeat("abcdefg", 30),
		Resources: nil,
	}
	vs := view.ViewState{
		LogModal: view.LogModal{
			TiltLog: true,
		},
	}

	rtf.run("tilt log", 70, 20, v, vs)
}

func TestRenderLogModal(t *testing.T) {
	rtf := newRendererTestFixture(t)

	vs := view.ViewState{
		Resources: []view.ResourceViewState{
			{},
		},
		LogModal: view.LogModal{ResourceLogNumber: 1},
	}

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:                "vigoda",
				LastBuildStartTime:  now.Add(-time.Minute),
				LastBuildFinishTime: now,
				LastBuildLog: `STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`,
				PodName:         "vigoda-pod",
				PodCreationTime: now,
				PodLog:          "serving on 8080",
				PodStatus:       "Running",
				LastDeployTime:  now,
			},
		},
	}
	rtf.run("build log pane", 117, 20, v, vs)
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

func TestRenderTiltfileError(t *testing.T) {
	rtf := newRendererTestFixture(t)
	v := view.View{
		TiltfileErrorMessage: "Tiltfile error!",
	}

	vs := view.ViewState{}

	rtf.run("tiltfile error", 60, 20, v, vs)
}

func TestAutoCollapseModes(t *testing.T) {
	rtf := newRendererTestFixture(t)

	goodView := view.View{
		Resources: []view.Resource{
			{
				Name:               "vigoda",
				DirectoriesWatched: []string{"bar"},
			},
		},
	}
	badView := view.View{
		Resources: []view.Resource{
			{
				Name:                "vigoda",
				DirectoriesWatched:  []string{"bar"},
				LastBuildFinishTime: time.Now(),
				LastBuildError:      "oh no the build failed",
				LastBuildLog:        "1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n",
			},
		},
	}

	autoVS := view.ViewState{
		Resources: []view.ResourceViewState{{CollapseState: view.CollapseAuto}},
	}
	collapseYesVS := view.ViewState{
		Resources: []view.ResourceViewState{{CollapseState: view.CollapseYes}},
	}
	collapseNoVS := view.ViewState{
		Resources: []view.ResourceViewState{{CollapseState: view.CollapseNo}},
	}
	rtf.run("collapse-auto-good", 70, 20, goodView, autoVS)
	rtf.run("collapse-auto-bad", 70, 20, badView, autoVS)
	rtf.run("collapse-no-good", 70, 20, goodView, collapseNoVS)
	rtf.run("collapse-yes-bad", 70, 20, badView, collapseYesVS)
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
