package hud

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/rty"

	"github.com/gdamore/tcell"
)

var clockForTest = func() time.Time { return time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC) }

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

	plainVs := fakeViewState(1, view.CollapseNo)

	rtf.run("one undeployed resource", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildStatus{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log:        []byte("1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n"),
				}},
			},
		},
	}
	rtf.run("inline build log", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildStatus{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log: []byte(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
    ╎ RUNNING: go install github.com/windmilleng/servantes/snack

    ╎ ERROR IN: go install github.com/windmilleng/servantes/snack
    ╎   → # github.com/windmilleng/servantes/snack
src/github.com/windmilleng/servantes/snack/main.go:16:36: syntax error: unexpected newline, expecting comma or }

ERROR: ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code 2`),
				}},
			},
		},
	}
	rtf.run("inline build log with wrapping", 117, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildStatus{{
					Error: fmt.Errorf("oh no the build failed"),
					Log:   []byte("1\n2\n3\nthe compiler wasn't smart enough to figure out what you meant!\n5\n6\n7\n8\n"),
				}},
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
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildStatus{{
					Error: fmt.Errorf("broken go code!"),
					Log:   []byte("mashing keys is not a good way to generate code"),
				}},
			},
		},
	}
	rtf.run("manifest error and build error", 70, 20, v, plainVs)

	ts := time.Now().Add(-5 * time.Minute)
	v = view.View{
		Resources: []view.Resource{
			{
				Name:               "a-a-a-aaaaabe vigoda",
				DirectoriesWatched: []string{"foo", "bar"},
				LastDeployTime:     ts,
				BuildHistory: []model.BuildStatus{{
					Edits:      []string{"main.go", "cli.go"},
					Error:      fmt.Errorf("the build failed!"),
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				PendingBuildEdits: []string{"main.go", "cli.go", "vigoda.go"},
				PendingBuildSince: ts,
				CurrentBuild: model.BuildStatus{
					Edits:     []string{"main.go"},
					StartTime: ts,
				},
				PodName:         "vigoda-pod",
				PodCreationTime: ts,
				PodStatus:       "Running",
				PodRestarts:     1,
				Endpoints:       []string{"1.2.3.4:8080"},
				PodLog:          "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("all the data at once", 70, 20, v, plainVs)
	rtf.run("all the data at once 50w", 50, 20, v, plainVs)
	rtf.run("all the data at once 10w", 10, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:               "abe vigoda",
				DirectoriesWatched: []string{"foo", "bar"},
				LastDeployTime:     ts,
				BuildHistory: []model.BuildStatus{{
					Edits: []string{"main.go"},
				}},
				PendingBuildSince: ts,
				CurrentBuild: model.BuildStatus{
					StartTime: ts,
					Reason:    model.BuildReasonFlagCrash,
				},
				PodName:         "vigoda-pod",
				PodCreationTime: ts,
				PodStatus:       "Running",
				PodRestarts:     0,
				Endpoints:       []string{"1.2.3.4:8080"},
				CrashLog:        "1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n",
			},
		},
	}
	rtf.run("crash rebuild", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:               "vigoda",
				DirectoriesWatched: []string{"foo", "bar"},
				LastDeployTime:     ts,
				BuildHistory: []model.BuildStatus{{
					Edits:      []string{"main.go", "cli.go"},
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				PodName:         "vigoda-pod",
				PodCreationTime: ts,
				PodStatus:       "Running",
				PodRestarts:     1,
				Endpoints:       []string{"1.2.3.4:8080"},
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
				Name: "GlobalYAML",
				BuildHistory: []model.BuildStatus{{
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				LastDeployTime: ts,
				IsYAMLManifest: true,
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
				Name: "vigoda",
				CurrentBuild: model.BuildStatus{
					StartTime: ts.Add(-5 * time.Second),
					Edits:     []string{"main.go"},
				},
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
				Name:           "vigoda",
				LastDeployTime: ts.Add(-5 * time.Second),
				BuildHistory: []model.BuildStatus{{
					Edits: []string{"abbot.go", "costello.go", "harold.go"},
				}},
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
	vs := fakeViewState(0, view.CollapseNo)
	vs.LogModal = view.LogModal{
		TiltLog: view.TiltLogFullScreen,
	}

	rtf.run("tilt log", 70, 20, v, vs)
}

func TestRenderLogModal(t *testing.T) {
	rtf := newRendererTestFixture(t)

	vs := fakeViewState(1, view.CollapseNo)
	vs.LogModal = view.LogModal{ResourceLogNumber: 1}

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildStatus{{
					StartTime:  now.Add(-time.Minute),
					FinishTime: now,
					Log: []byte(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`),
				}},
				PodName:         "vigoda-pod",
				PodCreationTime: now,
				PodLog:          "serving on 8080",
				PodStatus:       "Running",
				LastDeployTime:  now,
			},
		},
	}
	rtf.run("build log pane", 117, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildStatus{{
					FinishTime: now.Add(-time.Minute),
				}},
				CurrentBuild: model.BuildStatus{
					StartTime: now,
					Log:       []byte("building!"),
					Reason:    model.BuildReasonFlagCrash,
				},
				PodName:         "vigoda-pod",
				PodCreationTime: now,
				CrashLog:        "panic!",
			},
		},
	}
	rtf.run("resource log during crash rebuild", 60, 20, v, vs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "spoonerisms",
				ResourceInfo: view.DCResourceInfo{
					ConfigPath: "docker-compose.yml",
					Status:     "building",
					Log:        "Hi hello I'm a docker compose log",
				},
			},
		},
	}
	rtf.run("docker compose log pane", 117, 20, v, vs)
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
				Name:               "vigoda",
				DirectoriesWatched: []string{"bar"},
				BuildHistory: []model.BuildStatus{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log:        []byte("1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n"),
				}},
			},
		},
	}

	autoVS := fakeViewState(1, view.CollapseAuto)
	collapseYesVS := fakeViewState(1, view.CollapseYes)
	collapseNoVS := fakeViewState(1, view.CollapseNo)
	rtf.run("collapse-auto-good", 70, 20, goodView, autoVS)
	rtf.run("collapse-auto-bad", 70, 20, badView, autoVS)
	rtf.run("collapse-no-good", 70, 20, goodView, collapseNoVS)
	rtf.run("collapse-yes-bad", 70, 20, badView, collapseYesVS)
}

func TestPodPending(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildStatus{{
					StartTime:  ts,
					FinishTime: ts,
					Log: []byte(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`),
				}},
				PodName:        "vigoda-pod",
				PodLog:         "serving on 8080",
				PodStatus:      "",
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)

	rtf.run("pending pod no status", 80, 20, v, vs)
	assert.Equal(t, cPending,
		NewResourceView(v.Resources[0], vs.Resources[0], model.TriggerAuto, false, clockForTest).statusColor())

	v.Resources[0].PodCreationTime = ts
	v.Resources[0].PodStatus = "Pending"
	rtf.run("pending pod pending status", 80, 20, v, vs)
	assert.Equal(t, cPending,
		NewResourceView(v.Resources[0], vs.Resources[0], model.TriggerAuto, false, clockForTest).statusColor())
}

func TestPodLogContainerUpdate(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				PodName:   "vigoda-pod",
				PodStatus: "Running",
				Endpoints: []string{"1.2.3.4:8080"},
				PodLog:    "Serving on 8080",
				BuildHistory: []model.BuildStatus{{
					Log:        []byte("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
				}},
				PodUpdateStartTime: ts,
				PodCreationTime:    ts.Add(-time.Minute),
				LastDeployTime:     ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	vs.LogModal = view.LogModal{ResourceLogNumber: 1}
	rtf.run("pod log for container update", 70, 20, v, vs)
}

func TestCrashingPodInlineCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				PodName:   "vigoda-pod",
				PodStatus: "Error",
				Endpoints: []string{"1.2.3.4:8080"},
				PodLog:    "Something's maybe wrong idk",
				CrashLog:  "Definitely borken",
				BuildHistory: []model.BuildStatus{{
					Log:        []byte("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
					Reason:     model.BuildReasonFlagCrash,
				}},
				PodUpdateStartTime: ts,
				PodCreationTime:    ts.Add(-time.Minute),
				LastDeployTime:     ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("crashing pod displays crash log inline if present", 70, 20, v, vs)
}

func TestCrashingPodInlinePodLogIfNoCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				PodName:   "vigoda-pod",
				PodStatus: "Error",
				Endpoints: []string{"1.2.3.4:8080"},
				PodLog:    "Something's maybe wrong idk",
				BuildHistory: []model.BuildStatus{{
					Log:        []byte("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
					Reason:     model.BuildReasonFlagCrash,
				}},
				PodUpdateStartTime: ts,
				PodCreationTime:    ts.Add(-time.Minute),
				LastDeployTime:     ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("crashing pod displays pod log inline if no crash log if present", 70, 20, v, vs)
}

func TestNonCrashingPodNoInlineCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				PodName:   "vigoda-pod",
				PodStatus: "Running",
				Endpoints: []string{"1.2.3.4:8080"},
				PodLog:    "Something's maybe wrong idk",
				CrashLog:  "Definitely borken",
				BuildHistory: []model.BuildStatus{{
					Log:        []byte("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
				}},
				PodUpdateStartTime: ts,
				PodCreationTime:    ts.Add(-time.Minute),
				LastDeployTime:     ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("non-crashing pod displays no logs inline even if crash log if present", 70, 20, v, vs)
}

func TestPendingBuildInManualTriggerMode(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)
	v := view.View{
		TriggerMode: model.TriggerManual,
		Resources: []view.Resource{
			{
				Name:              "vigoda",
				PendingBuildSince: ts.Add(-5 * time.Second),
				PendingBuildEdits: []string{"main.go"},
			},
		},
	}
	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("pending build with manual trigger", 80, 20, v, vs)
}

func TestBuildHistory(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				PodName:   "vigoda-pod",
				PodStatus: "Running",
				BuildHistory: []model.BuildStatus{
					{
						Edits:      []string{"main.go"},
						StartTime:  ts.Add(-10 * time.Second),
						FinishTime: ts,
					},
					{
						Reason:     model.BuildReasonFlagInit,
						StartTime:  ts.Add(-2 * time.Minute),
						FinishTime: ts.Add(-2 * time.Minute).Add(5 * time.Second),
					},
				},
				PodUpdateStartTime: ts,
				PodCreationTime:    ts.Add(-time.Minute),
				LastDeployTime:     ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("multiple build history entries", 80, 20, v, vs)
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
	r := NewRenderer(clockForTest)
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""))
	c := r.layout(v, vs)
	rtf.i.Run(name, w, h, c)
}

var screen tcell.Screen

func TestMain(m *testing.M) {
	rty.InitScreenAndRun(m, &screen)
}

func fakeViewState(count int, collapse view.CollapseState) view.ViewState {
	vs := view.ViewState{}
	for i := 0; i < count; i++ {
		vs.Resources = append(vs.Resources, view.ResourceViewState{
			CollapseState: collapse,
		})
	}
	return vs
}
