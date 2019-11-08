package hud

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/gdamore/tcell"
)

const testCID = container.ID("beep-boop")

var clockForTest = func() time.Time { return time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC) }

func TestRender(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:               "foo",
				DirectoriesWatched: []string{"bar"},
				ResourceInfo:       view.K8sResourceInfo{},
			},
		},
	}

	plainVs := fakeViewState(1, view.CollapseNo)

	rtf.run("one undeployed resource", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildRecord{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log:        model.NewLog("1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n"),
				}},
				ResourceInfo: view.K8sResourceInfo{},
			},
		},
	}
	rtf.run("inline build log", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildRecord{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log: model.NewLog(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
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
				ResourceInfo: view.K8sResourceInfo{},
			},
		},
	}
	rtf.run("inline build log with wrapping", 117, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:      "a-a-a-aaaaabe vigoda",
				Endpoints: []string{"1.2.3.4:8080"},
				ResourceInfo: view.K8sResourceInfo{
					PodName:     "vigoda-pod",
					PodStatus:   "Running",
					PodRestarts: 1,
					PodLog:      model.NewLog("1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n"),
				},
			},
		},
	}

	rtf.run("pod log displayed inline", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "a-a-a-aaaaabe vigoda",
				BuildHistory: []model.BuildRecord{{
					Error: fmt.Errorf("broken go code!"),
					Log:   model.NewLog("mashing keys is not a good way to generate code"),
				}},
				ResourceInfo: view.K8sResourceInfo{},
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
				BuildHistory: []model.BuildRecord{{
					Edits:      []string{"main.go", "cli.go"},
					Error:      fmt.Errorf("the build failed!"),
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				PendingBuildEdits: []string{"main.go", "cli.go", "vigoda.go"},
				PendingBuildSince: ts,
				CurrentBuild: model.BuildRecord{
					Edits:     []string{"main.go"},
					StartTime: ts,
				},
				Endpoints: []string{"1.2.3.4:8080"},
				ResourceInfo: view.K8sResourceInfo{
					PodName:         "vigoda-pod",
					PodCreationTime: ts,
					PodStatus:       "Running",
					PodRestarts:     1,
					PodLog:          model.NewLog("1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n"),
				},
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
				BuildHistory: []model.BuildRecord{{
					Edits: []string{"main.go"},
				}},
				PendingBuildSince: ts,
				CurrentBuild: model.BuildRecord{
					StartTime: ts,
					Reason:    model.BuildReasonFlagCrash,
				},
				ResourceInfo: view.K8sResourceInfo{
					PodName:         "vigoda-pod",
					PodCreationTime: ts,
					PodStatus:       "Running",
					PodRestarts:     0,
				},
				Endpoints: []string{"1.2.3.4:8080"},
				CrashLog:  model.NewLog("1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n"),
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
				BuildHistory: []model.BuildRecord{{
					Edits:      []string{"main.go", "cli.go"},
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:         "vigoda-pod",
					PodCreationTime: ts,
					PodStatus:       "Running",
					PodRestarts:     1,
					PodLog: model.NewLog(`abe vigoda is crashing
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo`),
				},
				Endpoints: []string{"1.2.3.4:8080"},
			},
		},
	}
	rtf.run("pod log with inline wrapping", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: model.UnresourcedYAMLManifestName,
				BuildHistory: []model.BuildRecord{{
					FinishTime: ts,
					StartTime:  ts.Add(-1400 * time.Millisecond),
				}},
				LastDeployTime: ts,
				ResourceInfo: view.YAMLResourceInfo{
					K8sResources: []string{"sancho:deployment"},
				},
			},
		},
	}
	rtf.run("unresourced yaml manifest", 70, 20, v, plainVs)

	alertVs := plainVs
	alertVs.AlertMessage = "this is only a test"
	rtf.run("alert message", 70, 20, v, alertVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				CurrentBuild: model.BuildRecord{
					StartTime: ts.Add(-5 * time.Second),
					Edits:     []string{"main.go"},
				},
				ResourceInfo: view.K8sResourceInfo{},
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
				ResourceInfo:      view.K8sResourceInfo{},
			},
		},
	}
	rtf.run("pending build", 70, 20, v, plainVs)

	v = view.View{
		Resources: []view.Resource{
			{
				Name:           "vigoda",
				LastDeployTime: ts.Add(-5 * time.Second),
				BuildHistory: []model.BuildRecord{{
					Edits: []string{"abbot.go", "costello.go", "harold.go"},
				}},
				ResourceInfo: view.K8sResourceInfo{},
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
		Log:       model.NewLog(strings.Repeat("abcdefg", 30)),
		Resources: nil,
	}
	vs := fakeViewState(0, view.CollapseNo)

	rtf.run("tilt log", 70, 20, v, vs)

	vs.TiltLogState = view.TiltLogHalfScreen
	rtf.run("tilt log half screen", 70, 20, v, vs)

	vs.TiltLogState = view.TiltLogFullScreen
	rtf.run("tilt log full screen", 70, 20, v, vs)
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

func TestAutoCollapseModes(t *testing.T) {
	rtf := newRendererTestFixture(t)

	goodView := view.View{
		Resources: []view.Resource{
			{
				Name:               "vigoda",
				DirectoriesWatched: []string{"bar"},
				ResourceInfo:       view.K8sResourceInfo{},
			},
		},
	}
	badView := view.View{
		Resources: []view.Resource{
			{
				Name:               "vigoda",
				DirectoriesWatched: []string{"bar"},
				BuildHistory: []model.BuildRecord{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log:        model.NewLog("1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n"),
				}},
				ResourceInfo: view.K8sResourceInfo{},
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
				BuildHistory: []model.BuildRecord{{
					StartTime:  ts,
					FinishTime: ts,
					Log: model.NewLog(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`),
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:   "vigoda-pod",
					PodLog:    model.NewLog("serving on 8080"),
					PodStatus: "",
				},
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)

	rtf.run("pending pod no status", 80, 20, v, vs)
	assert.Equal(t, cPending, statusColor(v.Resources[0]))

	v.Resources[0].ResourceInfo = view.K8sResourceInfo{
		PodCreationTime: ts,
		PodStatus:       "Pending",
	}
	rtf.run("pending pod pending status", 80, 20, v, vs)
	assert.Equal(t, cPending, statusColor(v.Resources[0]))
}

func TestCrashingPodInlineCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				Endpoints: []string{"1.2.3.4:8080"},
				CrashLog:  model.NewLog("Definitely borken"),
				BuildHistory: []model.BuildRecord{{
					Log:        model.NewLog("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
					Reason:     model.BuildReasonFlagCrash,
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:            "vigoda-pod",
					PodStatus:          "Error",
					PodLog:             model.NewLog("Something's maybe wrong idk"),
					PodUpdateStartTime: ts,
					PodCreationTime:    ts.Add(-time.Minute),
				},
				LastDeployTime: ts,
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
				Endpoints: []string{"1.2.3.4:8080"},
				BuildHistory: []model.BuildRecord{{
					Log:        model.NewLog("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
					Reason:     model.BuildReasonFlagCrash,
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:            "vigoda-pod",
					PodStatus:          "Error",
					PodLog:             model.NewLog("Something's maybe wrong idk"),
					PodUpdateStartTime: ts,
					PodCreationTime:    ts.Add(-time.Minute),
				},
				LastDeployTime: ts,
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
				Endpoints: []string{"1.2.3.4:8080"},
				CrashLog:  model.NewLog("Definitely borken"),
				BuildHistory: []model.BuildRecord{{
					Log:        model.NewLog("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:            "vigoda-pod",
					PodStatus:          "Running",
					PodLog:             model.NewLog("Something's maybe wrong idk"),
					PodUpdateStartTime: ts,
					PodCreationTime:    ts.Add(-time.Minute),
				},
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("non-crashing pod displays no logs inline even if crash log if present", 70, 20, v, vs)
}

func TestCompletedPod(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:      "vigoda",
				Endpoints: []string{"1.2.3.4:8080"},
				BuildHistory: []model.BuildRecord{{
					Log:        model.NewLog("Building (1/2)\nBuilding (2/2)\n"),
					StartTime:  ts,
					FinishTime: ts,
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:            "vigoda-pod",
					PodStatus:          "Completed",
					PodUpdateStartTime: ts,
					PodCreationTime:    ts.Add(-time.Minute),
				},
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("Completed is a good status", 70, 20, v, vs)
}

func TestBrackets(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := view.View{
		Log: model.NewLog(`[build] This line should be prefixed with 'build'
[hello world] This line should be prefixed with [hello world]
[hello world] this line too
`),
		Resources: []view.Resource{
			{
				Name: "[vigoda]",
				BuildHistory: []model.BuildRecord{{
					StartTime:  ts,
					FinishTime: ts,
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:         "vigoda-pod",
					PodStatus:       "Running",
					PodCreationTime: ts,
				},
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseNo)

	rtf.run("text in brackets", 80, 20, v, vs)
}

func TestPendingBuildInManualTriggerMode(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)
	v := view.View{
		Resources: []view.Resource{
			{
				Name:              "vigoda",
				PendingBuildSince: ts.Add(-5 * time.Second),
				PendingBuildEdits: []string{"main.go"},
				TriggerMode:       model.TriggerModeManualAfterInitial,
				ResourceInfo:      view.K8sResourceInfo{},
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
				Name: "vigoda",
				BuildHistory: []model.BuildRecord{
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
				ResourceInfo: view.K8sResourceInfo{
					PodName:            "vigoda-pod",
					PodStatus:          "Running",
					PodUpdateStartTime: ts,
					PodCreationTime:    ts.Add(-time.Minute),
				},
				LastDeployTime: ts,
			},
		},
	}
	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("multiple build history entries", 80, 20, v, vs)
}

func TestDockerComposeUpExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:         "snack",
				ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusUp, testCID, model.NewLog("hellllo"), now.Add(-5*time.Second)),
				Endpoints:    []string{"http://localhost:3000"},
				CurrentBuild: model.BuildRecord{
					StartTime: now.Add(-5 * time.Second),
					Reason:    model.BuildReasonFlagChangedFiles,
				},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("docker-compose up expanded", 80, 20, v, vs)
}

func TestStatusBarDCRebuild(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:         "snack",
				ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusDown, testCID, model.NewLog("hellllo"), now.Add(-5*time.Second)),
				CurrentBuild: model.BuildRecord{
					StartTime: now.Add(-5 * time.Second),
					Reason:    model.BuildReasonFlagChangedFiles,
				},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseYes)
	rtf.run("status bar after intentional DC restart", 60, 20, v, vs)
}

func TestDetectDCCrashExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:         "snack",
				ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, model.NewLog("hi im a crash"), now.Add(-5*time.Second)),
			},
		},
	}

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("detected docker compose build crash expanded", 80, 20, v, vs)
}

func TestDetectDCCrashNotExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:         "snack",
				ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, model.NewLog("hi im a crash"), now.Add(-5*time.Second)),
			},
		},
	}

	vs := fakeViewState(1, view.CollapseYes)
	rtf.run("detected docker compose build crash not expanded", 80, 20, v, vs)
}

func TestDetectDCCrashAutoExpand(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:         "snack",
				ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, model.NewLog("hi im a crash"), now.Add(-5*time.Second)),
			},
		},
	}

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("detected docker compose build crash auto expand", 80, 20, v, vs)
}

func TestTiltfileResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Resources: []view.Resource{
			{
				Name:       store.TiltfileManifestName,
				IsTiltfile: true,
			},
		},
	}

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource", 80, 20, v, vs)
}

func TestTiltfileResourceWithWarning(t *testing.T) {
	rtf := newRendererTestFixture(t)
	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:       store.TiltfileManifestName,
				IsTiltfile: true,
				BuildHistory: []model.BuildRecord{
					{
						Edits:      []string{"Tiltfile"},
						StartTime:  now.Add(-5 * time.Second),
						FinishTime: now.Add(-4 * time.Second),
						Reason:     model.BuildReasonFlagConfig,
						Warnings:   []string{"I am warning you", "Something is alarming here"},
					},
				},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource with warning", 80, 20, v, vs)
}

func TestTiltfileResourcePending(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name:       store.TiltfileManifestName,
				IsTiltfile: true,
				CurrentBuild: model.BuildRecord{
					Edits:     []string{"Tiltfile"},
					StartTime: now.Add(-5 * time.Second),
					Reason:    model.BuildReasonFlagConfig,
					Log:       model.NewLog("Building..."),
				},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource pending", 80, 20, v, vs)
}

func TestRenderEscapedNbsp(t *testing.T) {
	rtf := newRendererTestFixture(t)
	plainVs := fakeViewState(1, view.CollapseNo)
	v := view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildRecord{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("oh no the build failed"),
					Log:        model.NewLog("\xa0 NBSP!"),
				}},
				ResourceInfo: view.K8sResourceInfo{},
			},
		},
	}
	rtf.run("escaped nbsp", 70, 20, v, plainVs)
}

func TestLineWrappingInInlineError(t *testing.T) {
	rtf := newRendererTestFixture(t)
	vs := fakeViewState(1, view.CollapseNo)
	lines := []string{}
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d: %s", i, strings.Repeat("xxx ", 20)))
	}
	v := view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildRecord{{
					FinishTime: time.Now(),
					Error:      fmt.Errorf("failure"),
					Log:        model.NewLog(strings.Join(lines, "\n")),
				}},
				ResourceInfo: view.K8sResourceInfo{},
			},
		},
	}
	rtf.run("line wrapping in inline error", 80, 40, v, vs)
}

func TestRenderTabView(t *testing.T) {
	rtf := newRendererTestFixture(t)

	vs := fakeViewState(1, view.CollapseAuto)
	now := time.Now()
	v := view.View{
		Resources: []view.Resource{
			{
				Name: "vigoda",
				BuildHistory: []model.BuildRecord{{
					StartTime:  now.Add(-time.Minute),
					FinishTime: now,
					Log: model.NewLog(`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`),
				}},
				ResourceInfo: view.K8sResourceInfo{
					PodName:         "vigoda-pod",
					PodCreationTime: now,
					PodLog:          model.NewLog("serving on 8080"),
					PodStatus:       "Running",
				},
				LastDeployTime: now,
			},
		},
	}
	v.Log = model.NewLog(fmt.Sprintf("%s\n%s\n",
		v.Resources[0].LastBuild().Log.String(),
		v.Resources[0].ResourceInfo.RuntimeLog()))

	rtf.run("log tab default", 117, 20, v, vs)

	vs.TabState = view.TabBuildLog
	rtf.run("log tab build", 117, 20, v, vs)

	vs.TabState = view.TabPodLog
	rtf.run("log tab pod", 117, 20, v, vs)
}

func TestPendingLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	ts := time.Now().Add(-5 * time.Minute)

	v := view.View{
		Resources: []view.Resource{
			{
				Name: "yarn-add",
				CurrentBuild: model.BuildRecord{
					StartTime: ts.Add(-5 * time.Second),
					Edits:     []string{"node.json"},
				},
				ResourceInfo: view.LocalResourceInfo{},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("unfinished local resource", 80, 20, v, vs)
}

func TestFinishedLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Resources: []view.Resource{
			{
				Name: "yarn-add",
				BuildHistory: []model.BuildRecord{
					model.BuildRecord{FinishTime: time.Now()},
				},
				ResourceInfo: view.LocalResourceInfo{},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("finished local resource", 80, 20, v, vs)
}

func TestErroredLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := view.View{
		Resources: []view.Resource{
			{
				Name: "yarn-add",
				BuildHistory: []model.BuildRecord{
					model.BuildRecord{
						FinishTime: time.Now(),
						Error:      fmt.Errorf("help i'm trapped in an error factory"),
						Log:        model.NewLog("1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n"),
					},
				},
				ResourceInfo: view.LocalResourceInfo{},
			},
		},
	}

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("errored local resource", 80, 20, v, vs)
}

type rendererTestFixture struct {
	i rty.InteractiveTester
}

func newRendererTestFixture(t rty.ErrorReporter) rendererTestFixture {
	return rendererTestFixture{
		i: rty.NewInteractiveTester(t, screen),
	}
}

func (rtf rendererTestFixture) run(name string, w int, h int, v view.View, vs view.ViewState) {
	// Assert that the view is serializable
	serialized, err := json.Marshal(v)
	if err != nil {
		rtf.i.T().Errorf("Malformed view: not serializable: %v\nView: %+q\n", err, v)
	}

	// Then, assert that the view can be marshaled back.
	if !json.Valid(serialized) {
		rtf.i.T().Errorf("Malformed view: bad serialization: %s", string(serialized))

	}

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
