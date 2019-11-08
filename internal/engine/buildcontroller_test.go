package engine

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/hud/server"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/podbuilder"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestBuildControllerOnePod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	pod := podbuilder.New(f.T(), manifest).Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, pod.Name, call.oneState().OneContainerInfo().PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerTooManyPodsForLiveUpdateErrorMessage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifest(f)
	f.Start([]model.Manifest{manifest}, true)

	// initial build
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	p1 := podbuilder.New(t, manifest).WithPodID("pod1").Build()
	p2 := podbuilder.New(t, manifest).WithPodID("pod2").Build()

	f.podEvent(p1, manifest.Name)
	f.podEvent(p2, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	// Should not have sent container info b/c too many pods
	assert.Equal(t, store.ContainerInfo{}, call.oneState().OneContainerInfo())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()

	err = call.oneState().RunningContainerError
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "can only get container info for a single pod",
			"should print error message when trying to get Running Containers for manifest with more than one pod")
	}
}

func TestBuildControllerTooManyPodsForDockerBuildNoErrorMessage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	f.Start([]model.Manifest{manifest}, true)

	// intial build
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	p1 := podbuilder.New(t, manifest).WithPodID("pod1").Build()
	p2 := podbuilder.New(t, manifest).WithPodID("pod2").Build()

	f.podEvent(p1, manifest.Name)
	f.podEvent(p2, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	// Should not have sent container info b/c too many pods
	assert.Equal(t, store.ContainerInfo{}, call.oneState().OneContainerInfo())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()

	// Should not have surfaced this log line b/c manifest doesn't have LiveUpdate instructions
	assert.NotContains(t, f.log.String(), "can only get container info for a single pod",
		"should print error message when trying to get Running Containers for manifest with more than one pod")
}

func TestBuildControllerIgnoresImageTags(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	ref := container.MustParseNamed("image-foo:tagged")
	manifest := f.newManifestWithRef("fe", ref)
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	pod := podbuilder.New(t, manifest).
		WithPodID("pod-id").
		WithImage("image-foo:othertag").
		Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneState().OneContainerInfo().PodID.String())

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
	assert.Equal(t, imageTarget, call.firstImgTarg())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	imageState := call.state[imageTarget.ID()]
	assert.Equal(t, "dc-sancho", imageState.OneContainerInfo().ContainerID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerLocalResource(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	dep := f.JoinPath("stuff.json")
	manifest := manifestbuilder.New(f, "yarn-add").
		WithLocalResource("echo beep boop", []string{dep}).Build()
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	lt := manifest.LocalTarget()
	assert.Equal(t, lt, call.local())

	f.fsWatcher.events <- watch.NewFileEvent(dep)

	call = f.nextCall()
	assert.Equal(t, lt, call.local())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerWontContainerBuildWithTwoPods(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	// Associate the pods with the manifest state
	podA := podbuilder.New(f.T(), manifest).WithPodID("pod-a").Build()
	podB := podbuilder.New(f.T(), manifest).WithPodID("pod-b").Build()
	f.podEvent(podA, manifest.Name)
	f.podEvent(podB, manifest.Name)

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	// We expect two pods associated with this manifest. We don't want to container-build
	// if there are multiple pods, so make sure we're not sending deploy info (i.e. that
	// we're doing an image build)
	call = f.nextCall()
	assert.Equal(t, "", call.oneState().OneContainerInfo().PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerTwoContainers(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	// container already on this pod matches the image built by this manifest
	pod := podbuilder.New(f.T(), manifest).Build()
	imgName := pod.Status.ContainerStatuses[0].Image
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        "same image",
		Image:       imgName, // matches image built by this manifest
		Ready:       true,
		ContainerID: "docker://cID-same-image",
	}, v1.ContainerStatus{
		Name:        "different image",
		Image:       "different-image", // does NOT match image built by this manifest
		Ready:       false,
		ContainerID: "docker://cID-different-image",
	})
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	runningContainers := call.oneState().RunningContainers

	require.Len(t, runningContainers, 2, "expect info for two containers (those "+
		"matching the image built by this manifest")

	c0 := runningContainers[0]
	c1 := runningContainers[1]

	assert.Equal(t, pod.Name, c0.PodID.String(), "pod ID for cInfo at index 0")
	assert.Equal(t, pod.Name, c1.PodID.String(), "pod ID for cInfo at index 1")

	assert.Equal(t, podbuilder.FakeContainerID(), c0.ContainerID, "container ID for cInfo at index 0")
	assert.Equal(t, "cID-same-image", c1.ContainerID.String(), "container ID for cInfo at index 1")

	assert.Equal(t, "sancho", c0.ContainerName.String(), "container name for cInfo at index 0")
	assert.Equal(t, "same image", c1.ContainerName.String(), "container name for cInfo at index 1")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerWontContainerBuildWithSomeButNotAllReadyContainers(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())

	// container already on this pod matches the image built by this manifest
	pod := podbuilder.New(f.T(), manifest).Build()
	imgName := pod.Status.ContainerStatuses[0].Image
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        "same image",
		Image:       imgName, // matches image built by this manifest
		Ready:       false,
		ContainerID: "docker://cID-same-image",
	})
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	// If even one of the containers matching this image is !ready, we have to do a
	// full rebuild, so don't return ANY RunningContainers.
	call = f.nextCall()
	runningContainers := call.oneState().RunningContainers
	assert.Empty(t, runningContainers)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerCrashRebuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneState().FilesChanged())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{podbuilder.FakeContainerID()}
	pb := podbuilder.New(f.T(), manifest)
	pod := pb.Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, pod.Name, call.oneState().OneContainerInfo().PodID.String())
	f.waitForCompletedBuildCount(2)
	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagChangedFiles, ms.LastBuild().Reason)
		assert.Equal(t, podbuilder.FakeContainerIDSet(1), ms.LiveUpdatedContainerIDs)
	})

	// Restart the pod with a new container id, to simulate a container restart.
	f.podEvent(pb.WithContainerID("funnyContainerID").Build(), manifest.Name)
	call = f.nextCall()
	assert.True(t, call.oneState().OneContainerInfo().Empty())
	f.waitForCompletedBuildCount(3)

	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestCrashRebuildTwoContainersOneImage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoTwoContainersOneImageYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{"c1", "c2"}
	f.podEvent(podbuilder.New(t, manifest).
		WithContainerIDAtIndex("c1", 0).
		WithContainerIDAtIndex("c2", 1).
		Build(), manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	f.waitForCompletedBuildCount(2)
	f.withManifestState("sancho", func(ms store.ManifestState) {
		assert.Equal(t, 2, len(ms.LiveUpdatedContainerIDs))
	})

	// Simulate pod event where one of the containers has been restarted with a new ID.
	f.podEvent(podbuilder.New(t, manifest).
		WithContainerID("c1").
		WithContainerIDAtIndex("c3", 1).
		Build(), manifest.Name)

	call = f.nextCall()
	f.waitForCompletedBuildCount(3)

	f.withManifestState("sancho", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestCrashRebuildTwoContainersTwoImages(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoTwoContainersOneImageYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		WithImageTarget(NewSanchoSidecarLiveUpdateImageTarget(f)).
		Build()
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	iTargs := call.imageTargets()
	require.Len(t, iTargs, 2)
	assert.Equal(t, manifest.ImageTargetAt(0), iTargs[0])
	assert.Equal(t, manifest.ImageTargetAt(1), iTargs[1])
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{"c1", "c2"}
	f.podEvent(podbuilder.New(t, manifest).
		WithContainerIDAtIndex("c1", 0).
		WithContainerIDAtIndex("c2", 1).
		Build(), manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	f.waitForCompletedBuildCount(2)
	f.withManifestState("sancho", func(ms store.ManifestState) {
		assert.Equal(t, 2, len(ms.LiveUpdatedContainerIDs))
	})

	// Simulate pod event where one of the containers has been restarted with a new ID.
	f.podEvent(podbuilder.New(t, manifest).
		WithContainerID("c1").
		WithContainerIDAtIndex("c3", 1).
		Build(), manifest.Name)

	call = f.nextCall()
	f.waitForCompletedBuildCount(3)

	f.withManifestState("sancho", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagCrash, ms.LastBuild().Reason)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestRecordLiveUpdatedContainerIDsForFailedLiveUpdate(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoTwoContainersOneImageYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	f.waitForCompletedBuildCount(1)

	expectedErr := fmt.Errorf("i can't let you do that dave")
	f.b.nextBuildFailure = expectedErr
	f.b.nextLiveUpdateContainerIDs = []container.ID{"c1", "c2"}

	f.podEvent(podbuilder.New(t, manifest).
		WithContainerIDAtIndex("c1", 0).
		WithContainerIDAtIndex("c2", 1).
		Build(), manifest.Name)
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	f.waitForCompletedBuildCount(2)
	f.withManifestState("sancho", func(ms store.ManifestState) {
		// Manifest should have recorded last build as a failure, but
		// ALSO have recorded the LiveUpdatedContainerIDs
		require.Equal(t, expectedErr, ms.BuildHistory[0].Error)

		assert.Equal(t, 2, len(ms.LiveUpdatedContainerIDs))
	})
}

func TestBuildControllerManualTriggerWithFileChanges(t *testing.T) {
	for _, tc := range []struct {
		name        string
		triggerMode model.TriggerMode
	}{
		{"manual including initial", model.TriggerModeManualIncludingInitial},
		{"manual after initial", model.TriggerModeManualAfterInitial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixture(t)
			defer f.TearDown()
			mName := model.ManifestName("foobar")

			manifest := f.newManifest(mName.String()).WithTriggerMode(tc.triggerMode)
			manifests := []model.Manifest{manifest}
			f.Start(manifests, true)

			// if we expect an initial build from the manifest, wait for it to complete
			if manifest.TriggerMode.AutoInitial() {
				f.nextCallComplete()
			}

			f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))
			f.WaitUntil("pending change appears", func(st store.EngineState) bool {
				return len(st.BuildStatus(manifest.ImageTargetAt(0).ID()).PendingFileChanges) > 0
			})
			f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

			f.store.Dispatch(server.AppendToTriggerQueueAction{Name: mName})
			call := f.nextCallComplete()
			state := call.oneState()
			assert.Equal(t, []string{f.JoinPath("main.go")}, state.FilesChanged())
			assert.False(t, state.NeedsForceUpdate) // it was NOT a force update, b/c there were changed files

			f.WaitUntil("manifest removed from queue", func(st store.EngineState) bool {
				for _, mn := range st.TriggerQueue {
					if mn == mName {
						return false
					}
				}
				return true
			})
		})
	}
}

func TestBuildControllerManualTriggerWithoutFileChangesForceUpdates(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	mName := model.ManifestName("foobar")

	manifest := f.newManifest(mName.String())
	manifests := []model.Manifest{manifest}
	f.Start(manifests, true)

	f.nextCall()
	f.waitForCompletedBuildCount(1)

	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: mName})
	call := f.nextCall()
	state := call.oneState()
	assert.Empty(t, state.FilesChanged())
	assert.True(t, state.NeedsForceUpdate)
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

	m1 := f.newManifestWithRef("manifest1", container.MustParseNamed("manifest1")).
		WithTriggerMode(model.TriggerModeManualAfterInitial)
	m2 := f.newManifestWithRef("manifest2", container.MustParseNamed("manifest2")).
		WithTriggerMode(model.TriggerModeManualAfterInitial)
	m3 := f.newManifestWithRef("manifest3", container.MustParseNamed("manifest3")).
		WithTriggerMode(model.TriggerModeManualIncludingInitial)
	m4 := f.newManifestWithRef("manifest4", container.MustParseNamed("manifest4")).
		WithTriggerMode(model.TriggerModeManualIncludingInitial)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m4, m2, m3, m1}
	f.Start(manifests, true)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(m1.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m2.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m3.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m4.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest2"})
	time.Sleep(10 * time.Millisecond)
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest4"})

	for i, _ := range manifests {
		expName := fmt.Sprintf("manifest%d", i+1)
		call := f.nextCall()
		imgID := call.firstImgTarg().ID().String()
		if assert.True(t, strings.HasSuffix(imgID, expName),
			"expected to get manifest '%s' but instead got: '%s' (checking suffix for manifest name)", expName, imgID) {
			assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneState().FilesChanged(),
				"for manifest '%s", expName)
		}
	}
	f.waitForCompletedBuildCount(expectedInitialBuildCount + len(manifests))
}

// This test is tightly coupled with FastBuild, and needs to be
// rewritten to use DockerBuild
func TestBuildQueueAndAutobuildOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	// changes to this dir. will register with our manual manifests
	syncDirManual := model.Sync{LocalPath: f.JoinPath("dirManual/"), ContainerPath: "/go"}
	// changes to this dir. will register with our automatic manifests
	syncDirAuto := model.Sync{LocalPath: f.JoinPath("dirAuto/"), ContainerPath: "/go"}

	m1 := f.newFastBuildManifest("manifest1", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManualAfterInitial)
	m2 := f.newFastBuildManifest("manifest2", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManualAfterInitial)
	m3 := f.newFastBuildManifest("manifest3", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManualIncludingInitial)
	m4 := f.newFastBuildManifest("manifest4", []model.Sync{syncDirManual}).WithTriggerMode(model.TriggerModeManualIncludingInitial)
	m5 := f.newFastBuildManifest("manifest5", []model.Sync{syncDirAuto}).WithTriggerMode(model.TriggerModeAuto)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m5, m4, m2, m3, m1}
	f.Start(manifests, true)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("dirManual/main.go"))
	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(m1.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m2.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m3.ImageTargetAt(0).ID()).PendingFileChanges) > 0 &&
			len(st.BuildStatus(m4.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})
	f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")

	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest1"})
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest2"})
	// make our one auto-trigger manifest build - should be evaluated LAST, after
	// all the manual manifests waiting in the queue
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("dirAuto/main.go"))
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest4"})

	for i, _ := range manifests {
		call := f.nextCall()
		assert.True(t, strings.HasSuffix(call.firstImgTarg().ID().String(), fmt.Sprintf("manifest%d", i+1)))

		if i < 4 {
			assert.Equal(t, []string{f.JoinPath("dirManual/main.go")}, call.oneState().FilesChanged(), "for manifest %d", i+1)
		} else {
			// the automatic manifest
			assert.Equal(t, []string{f.JoinPath("dirAuto/main.go")}, call.oneState().FilesChanged(), "for manifest %d", i+1)
		}
	}
	f.waitForCompletedBuildCount(len(manifests) + expectedInitialBuildCount)
}

// any manifests without image targets should be deployed before any manifests WITH image targets
func TestBuildControllerNoBuildManifestsFirst(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifests := make([]model.Manifest, 10)
	for i := 0; i < 10; i++ {
		manifests[i] = f.newManifest(fmt.Sprintf("built%d", i+1))
	}

	for _, i := range []int{3, 7, 8} {
		manifests[i] = manifestbuilder.New(f, model.ManifestName(fmt.Sprintf("unbuilt%d", i+1))).
			WithK8sYAML("fake-yaml").
			Build()
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

func TestBuildControllerUnresourcedYAMLFirst(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifests := []model.Manifest{
		f.newManifest("built1"),
		f.newManifest("built2"),
		f.newManifest("built3"),
		f.newManifest("built4"),
	}

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).WithK8sYAML("fake-yaml").Build())
	f.Start(manifests, true)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
	}

	expectedBuildOrder := []string{
		model.UnresourcedYAMLManifestName.String(),
		"built1",
		"built2",
		"built3",
		"built4",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerRespectDockerComposeOrder(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	sancho := NewSanchoFastBuildDCManifest(f)
	redis := manifestbuilder.New(f, "redis").WithDockerCompose().Build()
	donQuixote := manifestbuilder.New(f, "don-quixote").WithDockerCompose().Build()
	manifests := []model.Manifest{redis, sancho, donQuixote}
	f.Start(manifests, true)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		observedBuildOrder = append(observedBuildOrder, call.dc().Name.String())
	}

	// If these were Kubernetes resources, we would try to deploy don-quixote
	// before sancho, because it doesn't have an image build.
	//
	// But this would be wrong, because Docker Compose has stricter ordering requirements, see:
	// https://docs.docker.com/compose/startup-order/
	expectedBuildOrder := []string{
		"redis",
		"sancho",
		"don-quixote",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerLocalResourcesBeforeClusterResources(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifests := []model.Manifest{
		f.newManifest("clusterBuilt1"),
		f.newManifest("clusterBuilt2"),
		manifestbuilder.New(f, "clusterUnbuilt").
			WithK8sYAML("fake-yaml").Build(),
		manifestbuilder.New(f, "local1").
			WithLocalResource("echo local1", nil).Build(),
		f.newManifest("clusterBuilt3"),
		manifestbuilder.New(f, "local2").
			WithLocalResource("echo local2", nil).Build(),
	}

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).WithK8sYAML("also-fake-yaml").Build())
	f.Start(manifests, true)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		if !call.k8s().Empty() {
			observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
			continue
		}
		observedBuildOrder = append(observedBuildOrder, call.local().Name.String())
	}

	expectedBuildOrder := []string{
		model.UnresourcedYAMLManifestName.String(),
		"local1", // local resource comes after UnresourcedYAML but before all cluster resources
		"local2",
		"clusterUnbuilt",
		"clusterBuilt1",
		"clusterBuilt2",
		"clusterBuilt3",
	}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

func TestBuildControllerResourceDeps(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	depGraph := map[string][]string{
		"a": {"e"},
		"b": {"e"},
		"c": {"d", "g"},
		"d": {},
		"e": {"d", "f"},
		"f": {"c"},
		"g": {},
	}

	var manifests []model.Manifest
	manifestsByName := make(map[string]model.Manifest)
	for name, deps := range depGraph {
		m := f.newManifest(name)
		for _, dep := range deps {
			m.ResourceDependencies = append(m.ResourceDependencies, model.ManifestName(dep))
		}
		manifests = append(manifests, m)
		manifestsByName[name] = m
	}

	f.Start(manifests, true)

	var observedOrder []string
	for i := range manifests {
		call := f.nextCall("%dth build. have built: %v", i, observedOrder)
		name := call.k8s().Name.String()
		observedOrder = append(observedOrder, name)
		pb := podbuilder.New(t, manifestsByName[name]).WithContainerReady(true)
		f.podEvent(pb.Build(), model.ManifestName(name))
	}

	var expectedManifests []string
	for name := range depGraph {
		expectedManifests = append(expectedManifests, name)
	}

	// make sure everything built
	require.ElementsMatch(t, expectedManifests, observedOrder)

	buildIndexes := make(map[string]int)
	for i, n := range observedOrder {
		buildIndexes[n] = i
	}

	// make sure it happened in an acceptable order
	for name, deps := range depGraph {
		for _, dep := range deps {
			require.Truef(t, buildIndexes[name] > buildIndexes[dep], "%s built before %s, contrary to resource deps", name, dep)
		}
	}
}

// normally, local builds go before k8s builds
// if the local build depends on the k8s build, the k8s build should go first
func TestBuildControllerResourceDepTrumpsLocalResourcePriority(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8sManifest := f.newManifest("foo")
	localManifest := manifestbuilder.New(f, "bar").
		WithLocalResource("echo bar", nil).
		WithResourceDeps("foo").Build()
	manifests := []model.Manifest{localManifest, k8sManifest}
	f.Start(manifests, true)

	var observedBuildOrder []string
	for i := 0; i < len(manifests); i++ {
		call := f.nextCall()
		if !call.k8s().Empty() {
			observedBuildOrder = append(observedBuildOrder, call.k8s().Name.String())
			pb := podbuilder.New(t, k8sManifest).WithContainerReady(true)
			f.podEvent(pb.Build(), k8sManifest.Name)
			continue
		}
		observedBuildOrder = append(observedBuildOrder, call.local().Name.String())
	}

	expectedBuildOrder := []string{"foo", "bar"}
	assert.Equal(t, expectedBuildOrder, observedBuildOrder)
}

// bar depends on foo, we build foo three times before marking it ready, and make sure bar waits
func TestBuildControllerResourceDepTrumpsInitialBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	foo := manifestbuilder.New(f, "foo").
		WithLocalResource("foo cmd", []string{f.JoinPath("foo")}).
		Build()
	bar := manifestbuilder.New(f, "bar").
		WithLocalResource("bar cmd", []string{f.JoinPath("bar")}).
		WithResourceDeps("foo").
		Build()
	manifests := []model.Manifest{foo, bar}
	f.b.nextBuildFailure = errors.New("failure")
	f.Start(manifests, true)

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	f.b.nextBuildFailure = errors.New("failure")
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	// now that the foo build has succeeded, bar should get queued
	call = f.nextCall()
	require.Equal(t, "bar", call.local().Name.String())
}

// bar depends on foo, we build foo three times before marking it ready, and make sure bar waits
func TestBuildControllerResourceDepTrumpsPendingBuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	foo := manifestbuilder.New(f, "foo").
		WithLocalResource("foo cmd", []string{f.JoinPath("foo")}).
		Build()
	bar := manifestbuilder.New(f, "bar").
		WithLocalResource("bar cmd", []string{f.JoinPath("bar")}).
		WithResourceDeps("foo").
		Build()

	manifests := []model.Manifest{bar, foo}
	f.b.nextBuildFailure = errors.New("failure")
	f.Start(manifests, true)

	// trigger a change for bar so that it would try to build if not for its resource dep
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("bar", "main.go"))

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.b.nextBuildFailure = errors.New("failure")
	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.events <- watch.NewFileEvent(f.JoinPath("foo", "main2.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	// since the foo build succeeded, bar should now queue
	call = f.nextCall()
	require.Equal(t, "bar", call.local().Name.String())
}
