package hud

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"

	"github.com/gdamore/tcell"
)

const testCID = container.ID("beep-boop")

var clockForTest = func() time.Time { return time.Date(2017, 1, 1, 12, 0, 0, 0, time.UTC) }

func newView(resources ...view.Resource) view.View {
	return view.View{
		LogReader: newLogReader(""),
		Resources: resources,
	}
}

func newSpanLogReader(mn model.ManifestName, spanID logstore.SpanID, msg string) logstore.Reader {
	logStore := logstore.NewLogStore()
	logStore.Append(testLogAction{mn: mn, spanID: spanID, time: time.Now(), msg: msg}, nil)
	return logstore.NewReader(&sync.RWMutex{}, logStore)
}

func newWarningLogReader(mn model.ManifestName, spanID logstore.SpanID, warnings []string) logstore.Reader {
	logStore := logstore.NewLogStore()
	for _, warning := range warnings {
		logStore.Append(testLogAction{
			mn:     mn,
			spanID: spanID,
			time:   time.Now(),
			msg:    warning,
			level:  logger.WarnLvl,
		}, nil)
	}
	return logstore.NewReader(&sync.RWMutex{}, logStore)
}

func appendSpanLog(logStore *logstore.LogStore, mn model.ManifestName, spanID logstore.SpanID, msg string) {
	logStore.Append(testLogAction{mn: mn, spanID: spanID, time: time.Now(), msg: msg}, nil)
}

func TestRender(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView(view.Resource{
		Name:               "foo",
		DirectoriesWatched: []string{"bar"},
		ResourceInfo:       view.K8sResourceInfo{},
	})

	plainVs := fakeViewState(1, view.CollapseNo)

	rtf.run("one undeployed resource", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name: "a-a-a-aaaaabe vigoda",
		BuildHistory: []model.BuildRecord{{
			FinishTime: time.Now(),
			Error:      fmt.Errorf("oh no the build failed"),
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	v.LogReader = newSpanLogReader("a-a-a-aaaaabe vigoda", "vigoda:1",
		"1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n")

	rtf.run("inline build log", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name: "a-a-a-aaaaabe vigoda",
		BuildHistory: []model.BuildRecord{{
			FinishTime: time.Now(),
			Error:      fmt.Errorf("oh no the build failed"),
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	v.LogReader = newSpanLogReader("a-a-a-aaaaabe vigoda", "vigoda:1",
		`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
    ╎ RUNNING: go install github.com/windmilleng/servantes/snack

    ╎ ERROR IN: go install github.com/windmilleng/servantes/snack
    ╎   → # github.com/windmilleng/servantes/snack
src/github.com/windmilleng/servantes/snack/main.go:16:36: syntax error: unexpected newline, expecting comma or }

ERROR: ImageBuild: executor failed running [/bin/sh -c go install github.com/windmilleng/servantes/snack]: exit code 2`)
	rtf.run("inline build log with wrapping", 117, 20, v, plainVs)

	v = newView(view.Resource{
		Name:      "a-a-a-aaaaabe vigoda",
		Endpoints: []string{"1.2.3.4:8080"},
		ResourceInfo: view.K8sResourceInfo{
			PodName:     "vigoda-pod",
			PodStatus:   "Running",
			PodRestarts: 1,
			SpanID:      "vigoda:pod",
		},
	})
	v.LogReader = newSpanLogReader("a-a-a-aaaaabe vigoda", "vigoda:pod",
		"1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n")

	rtf.run("pod log displayed inline", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name: "a-a-a-aaaaabe vigoda",
		BuildHistory: []model.BuildRecord{{
			Error:  fmt.Errorf("broken go code!"),
			SpanID: "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	v.LogReader = newSpanLogReader("a-a-a-aaaaabe vigoda", "vigoda:1",
		"mashing keys is not a good way to generate code")
	rtf.run("manifest error and build error", 70, 20, v, plainVs)

	ts := time.Now().Add(-5 * time.Minute)
	v = newView(view.Resource{
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
			SpanID:          "vigoda:pod",
		},
	})
	v.LogReader = newSpanLogReader("a-a-a-aaaaabe vigoda", "vigoda:pod",
		"1\n2\n3\n4\nabe vigoda is now dead\n5\n6\n7\n8\n")
	rtf.run("all the data at once", 70, 20, v, plainVs)
	rtf.run("all the data at once 50w", 50, 20, v, plainVs)
	rtf.run("all the data at once 10w", 10, 20, v, plainVs)

	v = newView(view.Resource{
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
	})
	rtf.run("crash rebuild", 70, 20, v, plainVs)

	v = newView(view.Resource{
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
			SpanID:          "vigoda:pod",
		},
		Endpoints: []string{"1.2.3.4:8080"},
	})
	v.LogReader = newSpanLogReader("vigoda", "vigoda:pod",
		`abe vigoda is crashing
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo nooooooooooo noooooooooooo nooooooooooo
oh noooooooooooooooooo nooooooooooo noooooooooooo nooooooooooo`)
	rtf.run("pod log with inline wrapping", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name: model.UnresourcedYAMLManifestName,
		BuildHistory: []model.BuildRecord{{
			FinishTime: ts,
			StartTime:  ts.Add(-1400 * time.Millisecond),
		}},
		LastDeployTime: ts,
		ResourceInfo: view.YAMLResourceInfo{
			K8sResources: []string{"sancho:deployment"},
		},
	})
	rtf.run("no collapse unresourced yaml manifest", 70, 20, v, plainVs)
	rtf.run("default collapse unresourced yaml manifest", 70, 20, v, fakeViewState(1, view.CollapseAuto))

	alertVs := plainVs
	alertVs.AlertMessage = "this is only a test"
	rtf.run("alert message", 70, 20, v, alertVs)

	v = newView(view.Resource{
		Name: "vigoda",
		CurrentBuild: model.BuildRecord{
			StartTime: ts.Add(-5 * time.Second),
			Edits:     []string{"main.go"},
		},
		ResourceInfo: view.K8sResourceInfo{},
	})
	rtf.run("build in progress", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name:              "vigoda",
		PendingBuildSince: ts.Add(-5 * time.Second),
		PendingBuildEdits: []string{"main.go"},
		ResourceInfo:      view.K8sResourceInfo{},
	})
	rtf.run("pending build", 70, 20, v, plainVs)

	v = newView(view.Resource{
		Name:           "vigoda",
		LastDeployTime: ts.Add(-5 * time.Second),
		BuildHistory: []model.BuildRecord{{
			Edits: []string{"abbot.go", "costello.go", "harold.go"},
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	rtf.run("edited files narrow term", 60, 20, v, plainVs)
	rtf.run("edited files normal term", 80, 20, v, plainVs)
	rtf.run("edited files wide term", 120, 20, v, plainVs)
}

func TestRenderTiltLog(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView()
	v.LogReader = newLogReader(strings.Repeat("abcdefg", 30))

	vs := fakeViewState(0, view.CollapseNo)

	rtf.run("tilt log", 70, 20, v, vs)

	vs.TiltLogState = view.TiltLogHalfScreen
	rtf.run("tilt log half screen", 70, 20, v, vs)

	vs.TiltLogState = view.TiltLogFullScreen
	rtf.run("tilt log full screen", 70, 20, v, vs)
}

func TestRenderNarrationMessage(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView()
	vs := view.ViewState{
		ShowNarration:    true,
		NarrationMessage: "hi mom",
	}

	rtf.run("narration message", 60, 20, v, vs)
}

func TestAutoCollapseModes(t *testing.T) {
	rtf := newRendererTestFixture(t)

	goodView := newView(view.Resource{
		Name:               "vigoda",
		DirectoriesWatched: []string{"bar"},
		ResourceInfo:       view.K8sResourceInfo{},
	})
	badView := newView(view.Resource{
		Name:               "vigoda",
		DirectoriesWatched: []string{"bar"},
		BuildHistory: []model.BuildRecord{{
			FinishTime: time.Now(),
			Error:      fmt.Errorf("oh no the build failed"),
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	badView.LogReader = newSpanLogReader("vigoda", "vigoda:1",
		"1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n")

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

	v := newView(view.Resource{
		Name: "vigoda",
		BuildHistory: []model.BuildRecord{{
			StartTime:  ts,
			FinishTime: ts,
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{
			PodName:   "vigoda-pod",
			SpanID:    "vigoda:pod",
			PodStatus: "",
		},
		LastDeployTime: ts,
	})
	logStore := logstore.NewLogStore()
	appendSpanLog(logStore, "vigoda", "vigoda:1", `STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`)
	appendSpanLog(logStore, "vigoda", "vigoda:pod", "serving on 8080")
	v.LogReader = logstore.NewReader(&sync.RWMutex{}, logStore)
	vs := fakeViewState(1, view.CollapseAuto)

	rtf.run("pending pod no status", 80, 20, v, vs)
	assert.Equal(t, statusDisplay{color: cPending, spinner: true},
		statusDisplayOptions(v.Resources[0]))

	v.Resources[0].ResourceInfo = view.K8sResourceInfo{
		PodCreationTime: ts,
		PodStatus:       "Pending",
	}
	rtf.run("pending pod pending status", 80, 20, v, vs)
	assert.Equal(t, statusDisplay{color: cPending, spinner: true},
		statusDisplayOptions(v.Resources[0]))
}

func TestCrashingPodInlineCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
		Name:      "vigoda",
		Endpoints: []string{"1.2.3.4:8080"},
		CrashLog:  model.NewLog("Definitely borken"),
		BuildHistory: []model.BuildRecord{{
			SpanID:     "vigoda:1",
			StartTime:  ts,
			FinishTime: ts,
			Reason:     model.BuildReasonFlagCrash,
		}},
		ResourceInfo: view.K8sResourceInfo{
			PodName:            "vigoda-pod",
			PodStatus:          "Error",
			SpanID:             "vigoda:pod",
			PodUpdateStartTime: ts,
			PodCreationTime:    ts.Add(-time.Minute),
		},
		LastDeployTime: ts,
	})

	logStore := logstore.NewLogStore()
	appendSpanLog(logStore, "vigoda", "vigoda:1",
		"Building (1/2)\nBuilding (2/2)\n")
	appendSpanLog(logStore, "vigoda", "vigoda:pod",
		"Something's maybe wrong idk")
	v.LogReader = logstore.NewReader(&sync.RWMutex{}, logStore)

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("crashing pod displays crash log inline if present", 70, 20, v, vs)
}

func TestCrashingPodInlinePodLogIfNoCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
		Name:      "vigoda",
		Endpoints: []string{"1.2.3.4:8080"},
		BuildHistory: []model.BuildRecord{{
			SpanID:     "vigoda:1",
			StartTime:  ts,
			FinishTime: ts,
			Reason:     model.BuildReasonFlagCrash,
		}},
		ResourceInfo: view.K8sResourceInfo{
			PodName:            "vigoda-pod",
			PodStatus:          "Error",
			SpanID:             "vigoda:pod",
			PodUpdateStartTime: ts,
			PodCreationTime:    ts.Add(-time.Minute),
		},
		LastDeployTime: ts,
	})

	logStore := logstore.NewLogStore()
	appendSpanLog(logStore, "vigoda", "vigoda:1",
		"Building (1/2)\nBuilding (2/2)\n")
	appendSpanLog(logStore, "vigoda", "vigoda:pod",
		"Something's maybe wrong idk")
	v.LogReader = logstore.NewReader(&sync.RWMutex{}, logStore)

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("crashing pod displays pod log inline if no crash log if present", 70, 20, v, vs)
}

func TestNonCrashingPodNoInlineCrashLog(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
		Name:      "vigoda",
		Endpoints: []string{"1.2.3.4:8080"},
		CrashLog:  model.NewLog("Definitely borken"),
		BuildHistory: []model.BuildRecord{{
			SpanID:     "vigoda:1",
			StartTime:  ts,
			FinishTime: ts,
		}},
		ResourceInfo: view.K8sResourceInfo{
			PodName:            "vigoda-pod",
			PodStatus:          "Running",
			SpanID:             "vigoda:pod",
			PodUpdateStartTime: ts,
			PodCreationTime:    ts.Add(-time.Minute),
		},
		LastDeployTime: ts,
	})

	logStore := logstore.NewLogStore()
	appendSpanLog(logStore, "vigoda", "vigoda:1",
		"Building (1/2)\nBuilding (2/2)\n")
	appendSpanLog(logStore, "vigoda", "vigoda:pod",
		"Something's maybe wrong idk")
	v.LogReader = logstore.NewReader(&sync.RWMutex{}, logStore)

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("non-crashing pod displays no logs inline even if crash log if present", 70, 20, v, vs)
}

func TestCompletedPod(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
		Name:      "vigoda",
		Endpoints: []string{"1.2.3.4:8080"},
		BuildHistory: []model.BuildRecord{{
			SpanID:     "vigoda:1",
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
	})
	v.LogReader = newSpanLogReader("vigoda", "vigoda:1",
		"Building (1/2)\nBuilding (2/2)\n")
	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("Completed is a good status", 70, 20, v, vs)
}

func TestBrackets(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
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
	})
	v.LogReader = newLogReader(`[build] This line should be prefixed with 'build'
[hello world] This line should be prefixed with [hello world]
[hello world] this line too
`)

	vs := fakeViewState(1, view.CollapseNo)

	rtf.run("text in brackets", 80, 20, v, vs)
}

func TestPendingBuildInManualTriggerMode(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)
	v := newView(view.Resource{
		Name:              "vigoda",
		PendingBuildSince: ts.Add(-5 * time.Second),
		PendingBuildEdits: []string{"main.go"},
		TriggerMode:       model.TriggerModeManualAfterInitial,
		ResourceInfo:      view.K8sResourceInfo{},
	})
	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("pending build with manual trigger", 80, 20, v, vs)
}

func TestBuildHistory(t *testing.T) {
	rtf := newRendererTestFixture(t)
	ts := time.Now().Add(-30 * time.Second)

	v := newView(view.Resource{
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
	})
	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("multiple build history entries", 80, 20, v, vs)
}

func TestDockerComposeUpExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:         "snack",
		ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusUp, testCID, "snack:dc", now.Add(-5*time.Second)),
		Endpoints:    []string{"http://localhost:3000"},
		CurrentBuild: model.BuildRecord{
			StartTime: now.Add(-5 * time.Second),
			Reason:    model.BuildReasonFlagChangedFiles,
		},
	})
	v.LogReader = newSpanLogReader("snack", "snack:dc", "hellllo")

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("docker-compose up expanded", 80, 20, v, vs)
}

func TestStatusBarDCRebuild(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:         "snack",
		ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusDown, testCID, "snack:dc", now.Add(-5*time.Second)),
		CurrentBuild: model.BuildRecord{
			StartTime: now.Add(-5 * time.Second),
			Reason:    model.BuildReasonFlagChangedFiles,
		},
	})
	v.LogReader = newSpanLogReader("snack", "snack:dc", "hellllo")

	vs := fakeViewState(1, view.CollapseYes)
	rtf.run("status bar after intentional DC restart", 60, 20, v, vs)
}

func TestDetectDCCrashExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:         "snack",
		ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, "snack:dc", now.Add(-5*time.Second)),
	})
	v.LogReader = newSpanLogReader("snack", "snack:dc", "hi im a crash")

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("detected docker compose build crash expanded", 80, 20, v, vs)
}

func TestDetectDCCrashNotExpanded(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:         "snack",
		ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, "snack:dc", now.Add(-5*time.Second)),
	})
	v.LogReader = newSpanLogReader("snack", "snack:dc", "hi im a crash")

	vs := fakeViewState(1, view.CollapseYes)
	rtf.run("detected docker compose build crash not expanded", 80, 20, v, vs)
}

func TestDetectDCCrashAutoExpand(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:         "snack",
		ResourceInfo: view.NewDCResourceInfo([]string{"foo"}, dockercompose.StatusCrash, testCID, "snack:dc", now.Add(-5*time.Second)),
	})
	v.LogReader = newSpanLogReader("snack", "snack:dc", "hi im a crash")

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("detected docker compose build crash auto expand", 80, 20, v, vs)
}

func TestTiltfileResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView(view.Resource{
		Name:       store.TiltfileManifestName,
		IsTiltfile: true,
	})

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource", 80, 20, v, vs)
}

func TestTiltfileResourceWithWarning(t *testing.T) {
	rtf := newRendererTestFixture(t)
	now := time.Now()
	v := newView(view.Resource{
		Name:       store.TiltfileManifestName,
		IsTiltfile: true,
		BuildHistory: []model.BuildRecord{
			{
				Edits:        []string{"Tiltfile"},
				StartTime:    now.Add(-5 * time.Second),
				FinishTime:   now.Add(-4 * time.Second),
				Reason:       model.BuildReasonFlagConfig,
				WarningCount: 2,
				SpanID:       "tiltfile:1",
			},
		},
	})
	v.LogReader = newWarningLogReader(
		store.TiltfileManifestName,
		"tiltfile:1",
		[]string{"I am warning you\n", "Something is alarming here\n"})

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource with warning", 80, 20, v, vs)
}

func TestTiltfileResourcePending(t *testing.T) {
	rtf := newRendererTestFixture(t)

	now := time.Now()
	v := newView(view.Resource{
		Name:       store.TiltfileManifestName,
		IsTiltfile: true,
		CurrentBuild: model.BuildRecord{
			Edits:     []string{"Tiltfile"},
			StartTime: now.Add(-5 * time.Second),
			Reason:    model.BuildReasonFlagConfig,
			SpanID:    "tiltfile:1",
		},
	})
	v.LogReader = newSpanLogReader(store.TiltfileManifestName, "tiltfile:1", "Building...")

	vs := fakeViewState(1, view.CollapseNo)
	rtf.run("Tiltfile resource pending", 80, 20, v, vs)
}

func TestRenderEscapedNbsp(t *testing.T) {
	rtf := newRendererTestFixture(t)
	plainVs := fakeViewState(1, view.CollapseNo)
	v := newView(view.Resource{
		Name: "vigoda",
		BuildHistory: []model.BuildRecord{{
			FinishTime: time.Now(),
			Error:      fmt.Errorf("oh no the build failed"),
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	v.LogReader = newSpanLogReader("vigoda", "vigoda:1", "\xa0 NBSP!")
	rtf.run("escaped nbsp", 70, 20, v, plainVs)
}

func TestLineWrappingInInlineError(t *testing.T) {
	rtf := newRendererTestFixture(t)
	vs := fakeViewState(1, view.CollapseNo)
	lines := []string{}
	for i := 0; i < 10; i++ {
		lines = append(lines, fmt.Sprintf("line %d: %s", i, strings.Repeat("xxx ", 20)))
	}
	v := newView(view.Resource{
		Name: "vigoda",
		BuildHistory: []model.BuildRecord{{
			FinishTime: time.Now(),
			Error:      fmt.Errorf("failure"),
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{},
	})
	v.LogReader = newSpanLogReader("vigoda", "vigoda:1", strings.Join(lines, "\n"))
	rtf.run("line wrapping in inline error", 80, 40, v, vs)
}

func TestRenderTabView(t *testing.T) {
	rtf := newRendererTestFixture(t)

	vs := fakeViewState(1, view.CollapseAuto)
	now := time.Now()
	v := newView(view.Resource{
		Name: "vigoda",
		BuildHistory: []model.BuildRecord{{
			StartTime:  now.Add(-time.Minute),
			FinishTime: now,
			SpanID:     "vigoda:1",
		}},
		ResourceInfo: view.K8sResourceInfo{
			PodName:         "vigoda-pod",
			PodCreationTime: now,
			PodStatus:       "Running",
			SpanID:          "vigoda:pod",
		},
		LastDeployTime: now,
	})
	logStore := logstore.NewLogStore()
	appendSpanLog(logStore, "vigoda", "vigoda:1",
		`STEP 1/2 — Building Dockerfile: [gcr.io/windmill-public-containers/servantes/snack]
  │ Tarring context…
  │ Applying via kubectl
    ╎ Created tarball (size: 11 kB)
  │ Building image
`)
	appendSpanLog(logStore, "vigoda", "vigoda:pod", "serving on 8080")
	v.LogReader = logstore.NewReader(&sync.RWMutex{}, logStore)

	rtf.run("log tab default", 117, 20, v, vs)

	vs.TabState = view.TabBuildLog
	rtf.run("log tab build", 117, 20, v, vs)

	vs.TabState = view.TabRuntimeLog
	rtf.run("log tab pod", 117, 20, v, vs)
}

func TestPendingLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	ts := time.Now().Add(-5 * time.Minute)

	v := newView(view.Resource{
		Name: "yarn-add",
		CurrentBuild: model.BuildRecord{
			StartTime: ts.Add(-5 * time.Second),
			Edits:     []string{"node.json"},
		},
		ResourceInfo: view.NewLocalResourceInfo(model.RuntimeStatusPending, 0, model.LogSpanID("rt1")),
	})

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("unfinished local resource", 80, 20, v, vs)
}

func TestFinishedLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView(view.Resource{
		Name: "yarn-add",
		BuildHistory: []model.BuildRecord{
			model.BuildRecord{FinishTime: time.Now()},
		},
		ResourceInfo: view.NewLocalResourceInfo(model.RuntimeStatusNotApplicable, 0, model.LogSpanID("rt1")),
	})

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("finished local resource", 80, 20, v, vs)
}

func TestFailedBuildLocalResource(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView(view.Resource{
		Name: "yarn-add",
		BuildHistory: []model.BuildRecord{
			model.BuildRecord{
				FinishTime: time.Now(),
				Error:      fmt.Errorf("help i'm trapped in an error factory"),
				SpanID:     "build:1",
			},
		},
		ResourceInfo: view.LocalResourceInfo{},
	})
	v.LogReader = newSpanLogReader("yarn-add", "build:1",
		"1\n2\n3\nthe compiler did not understand!\n5\n6\n7\n8\n")

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("failed build local resource", 80, 20, v, vs)
}

func TestLocalResourceErroredServe(t *testing.T) {
	rtf := newRendererTestFixture(t)

	v := newView(view.Resource{
		Name: "yarn-add",
		BuildHistory: []model.BuildRecord{
			model.BuildRecord{FinishTime: time.Now()},
		},
		ResourceInfo: view.NewLocalResourceInfo(model.RuntimeStatusError, 0, model.LogSpanID("rt1")),
	})

	vs := fakeViewState(1, view.CollapseAuto)
	rtf.run("local resource errored serve", 80, 20, v, vs)
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
	r.rty = rty.NewRTY(tcell.NewSimulationScreen(""), rtf.i.T())
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

func newLogReader(msg string) logstore.Reader {
	store := logstore.NewLogStoreForTesting(msg)
	return logstore.NewReader(&sync.RWMutex{}, store)
}

type testLogAction struct {
	mn     model.ManifestName
	spanID logstore.SpanID
	time   time.Time
	msg    string
	level  logger.Level
	fields logger.Fields
}

func (e testLogAction) Fields() logger.Fields {
	return e.fields
}

func (e testLogAction) Message() []byte {
	return []byte(e.msg)
}

func (e testLogAction) Level() logger.Level {
	if e.level == (logger.Level{}) {
		return logger.InfoLvl
	}
	return e.level
}

func (e testLogAction) Time() time.Time {
	return e.time
}

func (e testLogAction) ManifestName() model.ManifestName {
	return e.mn
}

func (e testLogAction) SpanID() logstore.SpanID {
	return e.spanID
}
