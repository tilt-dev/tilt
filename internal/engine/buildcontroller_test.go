package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestBuildControllerOnePod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Sync{sync})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, time.Now()))
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneState().DeployInfo.PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerIgnoresImageTags(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	ref := container.MustParseNamed("image-foo:tagged")
	manifest := f.newManifestWithRef("fe", ref, []model.Sync{sync})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	pod := f.testPod("pod-id", "fe", "Running", testContainer, time.Now())
	setImage(pod, "image-foo:othertag")
	f.podEvent(pod)
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneState().DeployInfo.PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerDockerCompose(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoFastBuildDCManifest(f)
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	imageTarget := manifest.ImageTargetAt(0)
	assert.Equal(t, imageTarget, call.image())

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	call = f.nextCall()
	imageState := call.state[imageTarget.ID()]
	assert.Equal(t, "dc-sancho", imageState.DeployInfo.ContainerID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerWontContainerBuildWithTwoPods(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Sync{sync})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	// Associate the pods with the manifest state
	podA := f.testPod("pod-a", "fe", "Running", testContainer, time.Now())
	podB := f.testPod("pod-b", "fe", "Running", testContainer, time.Now())
	f.podEvent(podA)
	f.podEvent(podB)

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	// We expect two pods associated with this manifest. We don't want to container-build
	// if there are multiple pods, so make sure we're not sending deploy info (i.e. that
	// we're doing an image build)
	call = f.nextCall()
	assert.Equal(t, "", call.oneState().DeployInfo.PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerCrashRebuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Sync{sync})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.image())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())
	f.waitForCompletedBuildCount(1)

	f.b.nextBuildContainer = testContainer
	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, time.Now()))
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneState().DeployInfo.PodID.String())
	f.waitForCompletedBuildCount(2)
	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagChangedFiles, ms.LastBuild().Reason)
		assert.Equal(t, testContainer, ms.ExpectedContainerID.String())
	})

	// Restart the pod with a new container id, to simulate a container restart.
	f.podEvent(f.testPod("pod-id", "fe", "Running", "funnyContainerID", time.Now()))
	call = f.nextCall()
	assert.True(t, call.oneState().DeployInfo.Empty())
	f.waitForCompletedBuildCount(3)

	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerManualTrigger(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mName := model.ManifestName("foobar")

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest(mName.String(), []model.Sync{sync}).WithTriggerMode(model.TriggerModeManual)
	f.Init(InitAction{
		Manifests:       []model.Manifest{manifest},
		WatchFiles:      true,
		ExecuteTiltfile: true,
	})

	f.nextCall()
	f.waitForCompletedBuildCount(1)

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: mName})
	f.assertNoCall("manifest has no pending changes, so shouldn't build even if we try to trigger it")

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(manifest.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: mName})
	call := f.nextCall()
	assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneState().FilesChanged())
	f.waitForCompletedBuildCount(2)

	f.WaitUntil("manifest removed from queue", func(st store.EngineState) bool {
		for _, mn := range st.TriggerQueue {
			if mn == mName {
				return false
			}
		}
		return true
	})
}

func TestBuildQueueOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
	m1 := f.newManifest("manifest1", []model.Sync{sync}).WithTriggerMode(model.TriggerModeManual)
	m2 := f.newManifest("manifest2", []model.Sync{sync}).WithTriggerMode(model.TriggerModeManual)
	m3 := f.newManifest("manifest3", []model.Sync{sync}).WithTriggerMode(model.TriggerModeManual)
	m4 := f.newManifest("manifest4", []model.Sync{sync}).WithTriggerMode(model.TriggerModeManual)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m4, m2, m3, m1}
	f.Init(InitAction{
		Manifests:       manifests,
		WatchFiles:      true,
		ExecuteTiltfile: true,
	})

	// Wait for initial build
	for _, _ = range manifests {
		f.nextCall()
	}
	f.waitForCompletedBuildCount(len(manifests))

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(m1.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m2.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m3.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m4.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest2"})
	time.Sleep(10 * time.Millisecond)
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest4"})

	for i, _ := range manifests {
		expName := fmt.Sprintf("manifest%d", i+1)
		call := f.nextCall()
		imgID := call.image().ID().String()
		if assert.True(t, strings.HasSuffix(imgID, expName),
			"expected to get manifest '%s' but instead got: '%s' (checking suffix for manifest name)", expName, imgID) {
			assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneState().FilesChanged(),
				"for manifest '%s", expName)
		}
	}
	f.waitForCompletedBuildCount(2 * len(manifests))
}

func TestBuildQueueAndAutobuildOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	// changes to this dir. will register with our manual manifests
	syncDirManual := model.Sync{LocalPath: f.JoinPath("dirManual/"), ContainerPath: "/go"}
	// changes to this dir. will register with our automatic manifests
	syncDirAuto := model.Sync{LocalPath: f.JoinPath("dirAuto/"), ContainerPath: "/go"}

	m1 := f.newManifest("manifest1", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManual)
	m2 := f.newManifest("manifest2", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManual)
	m3 := f.newManifest("manifest3", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManual)
	m4 := f.newManifest("manifest4", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManual)
	m5 := f.newManifest("manifest5", []model.Sync{syncDirAuto}).WithTriggerMode(model.TriggerModeAuto)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m5, m4, m2, m3, m1}
	f.Init(InitAction{
		Manifests:       manifests,
		WatchFiles:      true,
		ExecuteTiltfile: true,
	})

	// Wait for initial build
	for _, _ = range manifests {
		f.nextCall()
	}
	f.waitForCompletedBuildCount(len(manifests))

	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("dirManual/main.go")}
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(m1.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m2.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m3.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m4.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest2"})
	// make our one auto-trigger manifest build - should be evaluated LAST, after
	// all the manual manifests waiting in the queue
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("dirAuto/main.go")}
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "manifest4"})

	for i, _ := range manifests {
		call := f.nextCall()
		assert.True(t, strings.HasSuffix(call.image().ID().String(), fmt.Sprintf("manifest%d", i+1)))

		if i < 4 {
			assert.Equal(t, []string{f.JoinPath("dirManual/main.go")}, call.oneState().FilesChanged(), "for manifest %d", i+1)
		} else {
			// the automatic manifest
			assert.Equal(t, []string{f.JoinPath("dirAuto/main.go")}, call.oneState().FilesChanged(), "for manifest %d", i+1)
		}
	}
	f.waitForCompletedBuildCount(2 * len(manifests))
}

// any manifests without image targets should be deployed before any manifests WITH image targets
func TestBuildControllerNoBuildManifestsFirst(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifests := make([]model.Manifest, 10)
	for i := 0; i < 10; i++ {
		sync := model.Sync{LocalPath: f.Path(), ContainerPath: "/go"}
		manifests[i] = f.newManifest(fmt.Sprintf("built%d", i+1), []model.Sync{sync})
	}

	for _, i := range []int{3, 7, 8} {
		manifests[i] = assembleK8sManifest(
			model.Manifest{
				Name: model.ManifestName(fmt.Sprintf("unbuilt%d", i+1))},
			model.K8sTarget{YAML: "fake-yaml"})
	}
	f.Start(manifests, true)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
	}

	// throwing a bunch of elements at it to increase confidence we maintain order between built and unbuilt
	// this might miss bugs since we might just get these elements back in the right order via luck
	expectedBuildOrder := []string{
		"unbuilt4",
		"unbuilt8",
		"unbuilt9",
		"built1",
		"built2",
		"built3",
		"built5",
		"built6",
		"built7",
		"built10",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}
