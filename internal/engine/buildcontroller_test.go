package engine

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/watch"
)

func TestBuildControllerOnePod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	pod := podbuilder.New(f.T(), manifest).Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, pod.Name, call.oneImageState().OneContainerInfo().PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerTooManyPodsForLiveUpdateErrorMessage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateManifest(f)
	f.Start([]model.Manifest{manifest})

	// initial build
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	p1 := podbuilder.New(t, manifest).WithPodID("pod1").Build()
	p2 := podbuilder.New(t, manifest).WithPodID("pod2").Build()

	f.podEvent(p1, manifest.Name)
	f.podEvent(p2, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	// Should not have sent container info b/c too many pods
	assert.Equal(t, store.ContainerInfo{}, call.oneImageState().OneContainerInfo())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()

	err = call.oneImageState().RunningContainerError
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "can only get container info for a single pod",
			"should print error message when trying to get Running Containers for manifest with more than one pod")
	}
}

func TestBuildControllerTooManyPodsForDockerBuildNoErrorMessage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildManifest(f)
	f.Start([]model.Manifest{manifest})

	// initial build
	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	p1 := podbuilder.New(t, manifest).WithPodID("pod1").Build()
	p2 := podbuilder.New(t, manifest).WithPodID("pod2").Build()

	f.podEvent(p1, manifest.Name)
	f.podEvent(p2, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	// Should not have sent container info b/c too many pods
	assert.Equal(t, store.ContainerInfo{}, call.oneImageState().OneContainerInfo())

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	pod := podbuilder.New(t, manifest).
		WithPodID("pod-id").
		WithImage("image-foo:othertag").
		Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.oneImageState().OneContainerInfo().PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerDockerCompose(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := NewSanchoLiveUpdateDCManifest(f)
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	imageTarget := manifest.ImageTargetAt(0)
	assert.Equal(t, imageTarget, call.firstImgTarg())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	lt := manifest.LocalTarget()
	assert.Equal(t, lt, call.local())

	f.fsWatcher.Events <- watch.NewFileEvent(dep)

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	// Associate the pods with the manifest state
	podA := podbuilder.New(f.T(), manifest).WithPodID("pod-a").Build()
	podB := podbuilder.New(f.T(), manifest).WithPodID("pod-b").Build()
	f.podEvent(podA, manifest.Name)
	f.podEvent(podB, manifest.Name)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	// We expect two pods associated with this manifest. We don't want to container-build
	// if there are multiple pods, so make sure we're not sending deploy info (i.e. that
	// we're doing an image build)
	call = f.nextCall()
	assert.Equal(t, "", call.oneImageState().OneContainerInfo().PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerTwoContainers(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	// container already on this pod matches the image built by this manifest
	pod := podbuilder.New(f.T(), manifest).Build()
	imgName := pod.Status.ContainerStatuses[0].Image
	runningState := v1.ContainerState{
		Running: &v1.ContainerStateRunning{
			StartedAt: metav1.NewTime(time.Now()),
		},
	}
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, v1.ContainerStatus{
		Name:        "same image",
		Image:       imgName, // matches image built by this manifest
		Ready:       true,
		State:       runningState,
		ContainerID: "docker://cID-same-image",
	}, v1.ContainerStatus{
		Name:        "different image",
		Image:       "different-image", // does NOT match image built by this manifest
		Ready:       false,
		State:       runningState,
		ContainerID: "docker://cID-different-image",
	})
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	runningContainers := call.oneImageState().RunningContainers

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

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
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	// If even one of the containers matching this image is !ready, we have to do a
	// full rebuild, so don't return ANY RunningContainers.
	f.assertNoCall()

	f.withState(func(st store.EngineState) {
		_, holds := buildcontrol.NextTargetToBuild(st)
		assert.Equal(t, store.HoldWaitingForDeploy, holds[manifest.Name])
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerCrashRebuild(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{podbuilder.FakeContainerID()}
	pb := podbuilder.New(f.T(), manifest)
	pod := pb.Build()
	f.podEvent(pod, manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	assert.Equal(t, pod.Name, call.oneImageState().OneContainerInfo().PodID.String())
	f.waitForCompletedBuildCount(2)
	f.withManifestState("fe", func(ms store.ManifestState) {
		assert.Equal(t, model.BuildReasonFlagChangedFiles, ms.LastBuild().Reason)
		assert.Equal(t, podbuilder.FakeContainerIDSet(1), ms.LiveUpdatedContainerIDs)
	})

	// Restart the pod with a new container id, to simulate a container restart.
	f.podEvent(pb.WithContainerID("funnyContainerID").Build(), manifest.Name)
	call = f.nextCall()
	assert.True(t, call.oneImageState().OneContainerInfo().Empty())
	assert.False(t, call.oneImageState().FullBuildTriggered)
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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	f.waitForCompletedBuildCount(1)

	f.b.nextLiveUpdateContainerIDs = []container.ID{"c1", "c2"}
	f.podEvent(podbuilder.New(t, manifest).
		WithContainerIDAtIndex("c1", 0).
		WithContainerIDAtIndex("c2", 1).
		Build(), manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

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
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): investigate")
	}
	f := newTestFixture(t)
	defer f.TearDown()

	manifest := manifestbuilder.New(f, "sancho").
		WithK8sYAML(testyaml.SanchoTwoContainersOneImageYAML).
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		WithImageTarget(NewSanchoSidecarLiveUpdateImageTarget(f)).
		Build()
	f.Start([]model.Manifest{manifest})

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
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

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
	f.Start([]model.Manifest{manifest})

	call := f.nextCall()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	f.waitForCompletedBuildCount(1)

	expectedErr := fmt.Errorf("i can't let you do that dave")
	f.SetNextLiveUpdateCompileError(expectedErr, []container.ID{"c1", "c2"})

	f.podEvent(podbuilder.New(t, manifest).
		WithContainerIDAtIndex("c1", 0).
		WithContainerIDAtIndex("c2", 1).
		Build(), manifest.Name)
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))

	call = f.nextCall()
	f.waitForCompletedBuildCount(2)
	f.withManifestState("sancho", func(ms store.ManifestState) {
		// Manifest should have recorded last build as a failure, but
		// ALSO have recorded the LiveUpdatedContainerIDs
		require.Equal(t, expectedErr, ms.BuildHistory[0].Error)

		assert.Equal(t, 2, len(ms.LiveUpdatedContainerIDs))
	})
}

func TestBuildControllerManualTriggerBuildReasonInit(t *testing.T) {
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
			f.Start(manifests)

			// make sure there's a first build
			if !manifest.TriggerMode.AutoInitial() {
				f.store.Dispatch(server.AppendToTriggerQueueAction{Name: mName})
			}

			f.nextCallComplete()

			f.withManifestState(mName, func(ms store.ManifestState) {
				require.Equal(t, tc.triggerMode.AutoInitial(), ms.LastBuild().Reason.Has(model.BuildReasonFlagInit))
			})
		})
	}
}

func TestBuildControllerImageBuildTrigger(t *testing.T) {
	for _, tc := range []struct {
		name               string
		triggerMode        model.TriggerMode
		filesChanged       bool
		expectedImageBuild bool
	}{
		{name: "manual including initial with change", triggerMode: model.TriggerModeManualIncludingInitial, filesChanged: true, expectedImageBuild: false},
		{name: "manual after initial with change", triggerMode: model.TriggerModeManualAfterInitial, filesChanged: true, expectedImageBuild: false},
		{name: "manual including initial without change", triggerMode: model.TriggerModeManualIncludingInitial, filesChanged: false, expectedImageBuild: true},
		{name: "manual after initial without change", triggerMode: model.TriggerModeManualAfterInitial, filesChanged: false, expectedImageBuild: true},
		{name: "auto without change", triggerMode: model.TriggerModeAuto, filesChanged: false, expectedImageBuild: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTestFixture(t)
			defer f.TearDown()
			mName := model.ManifestName("foobar")

			manifest := f.newManifest(mName.String()).WithTriggerMode(tc.triggerMode)
			manifests := []model.Manifest{manifest}
			f.Start(manifests)

			// if we expect an initial build from the manifest, wait for it to complete
			if manifest.TriggerMode.AutoInitial() {
				f.nextCallComplete()
			}

			expectedFiles := []string{}
			if tc.filesChanged {
				expectedFiles = append(expectedFiles, f.JoinPath("main.go"))
				f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
			}
			f.WaitUntil("pending change appears", func(st store.EngineState) bool {
				return len(st.BuildStatus(manifest.ImageTargetAt(0).ID()).PendingFileChanges) >= len(expectedFiles)
			})

			if manifest.TriggerMode.AutoOnChange() {
				f.assertNoCall("even tho there are pending changes, manual manifest shouldn't build w/o explicit trigger")
			}

			f.store.Dispatch(server.AppendToTriggerQueueAction{Name: mName})
			call := f.nextCallComplete()
			state := call.oneImageState()
			assert.Equal(t, expectedFiles, state.FilesChanged())
			assert.Equal(t, tc.expectedImageBuild, state.FullBuildTriggered)

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

// it should be a force update if there have been no file changes since the last build
// make sure file changes prior to the last build are ignored for this purpose
func TestBuildControllerManualTriggerWithFileChangesSinceLastSuccessfulBuildButBeforeLastBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newTestFixture(t)
	defer f.TearDown()
	mName := model.ManifestName("foobar")

	manifest := f.newManifest(mName.String())
	manifests := []model.Manifest{manifest}
	f.Start(manifests)

	f.nextCallComplete()

	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.b.nextBuildError = errors.New("build failure!")
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
	f.nextCallComplete()

	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: mName})
	call := f.nextCallComplete()
	state := call.oneImageState()
	assert.Equal(t, []string{}, state.FilesChanged())
	assert.True(t, state.FullBuildTriggered)
	assert.True(t, call.k8sState().FullBuildTriggered)

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
	f.Start(manifests)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("main.go"))
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

	for i := range manifests {
		expName := fmt.Sprintf("manifest%d", i+1)
		call := f.nextCall()
		imgID := call.firstImgTarg().ID().String()
		if assert.True(t, strings.HasSuffix(imgID, expName),
			"expected to get manifest '%s' but instead got: '%s' (checking suffix for manifest name)", expName, imgID) {
			assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneImageState().FilesChanged(),
				"for manifest '%s", expName)
		}
	}
	f.waitForCompletedBuildCount(expectedInitialBuildCount + len(manifests))
}

func TestBuildQueueAndAutobuildOrdering(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	// changes to this dir. will register with our manual manifests
	dirManual := f.JoinPath("dirManual/")
	// changes to this dir. will register with our automatic manifests
	dirAuto := f.JoinPath("dirAuto/")

	m1 := f.newDockerBuildManifestWithBuildPath("manifest1", dirManual).WithTriggerMode(model.TriggerModeManualAfterInitial)
	m2 := f.newDockerBuildManifestWithBuildPath("manifest2", dirManual).WithTriggerMode(model.TriggerModeManualAfterInitial)
	m3 := f.newDockerBuildManifestWithBuildPath("manifest3", dirManual).WithTriggerMode(model.TriggerModeManualIncludingInitial)
	m4 := f.newDockerBuildManifestWithBuildPath("manifest4", dirManual).WithTriggerMode(model.TriggerModeManualIncludingInitial)
	m5 := f.newDockerBuildManifestWithBuildPath("manifest5", dirAuto).WithTriggerMode(model.TriggerModeAuto)

	// attach to state in different order than we plan to trigger them
	manifests := []model.Manifest{m5, m4, m2, m3, m1}
	f.Start(manifests)

	expectedInitialBuildCount := 0
	for _, m := range manifests {
		if m.TriggerMode.AutoInitial() {
			expectedInitialBuildCount++
			f.nextCall()
		}
	}

	f.waitForCompletedBuildCount(expectedInitialBuildCount)

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("dirManual/main.go"))
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
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("dirAuto/main.go"))
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest3"})
	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: "manifest4"})

	for i := range manifests {
		call := f.nextCall()
		assert.True(t, strings.HasSuffix(call.firstImgTarg().ID().String(), fmt.Sprintf("manifest%d", i+1)))

		if i < 4 {
			assert.Equal(t, []string{f.JoinPath("dirManual/main.go")}, call.oneImageState().FilesChanged(), "for manifest %d", i+1)
		} else {
			// the automatic manifest
			assert.Equal(t, []string{f.JoinPath("dirAuto/main.go")}, call.oneImageState().FilesChanged(), "for manifest %d", i+1)
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
			WithK8sYAML(SanchoYAML).
			Build()
	}
	f.Start(manifests)

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

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).Build())
	f.Start(manifests)

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

	sancho := NewSanchoLiveUpdateDCManifest(f)
	redis := manifestbuilder.New(f, "redis").WithDockerCompose().Build()
	donQuixote := manifestbuilder.New(f, "don-quixote").WithDockerCompose().Build()
	manifests := []model.Manifest{redis, sancho, donQuixote}
	f.Start(manifests)

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
			WithK8sYAML(SanchoYAML).Build(),
		manifestbuilder.New(f, "local1").
			WithLocalResource("echo local1", nil).Build(),
		f.newManifest("clusterBuilt3"),
		manifestbuilder.New(f, "local2").
			WithLocalResource("echo local2", nil).Build(),
	}

	manifests = append(manifests, manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).Build())
	f.Start(manifests)

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
		"local1",
		"local2",
		model.UnresourcedYAMLManifestName.String(),
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

	f.Start(manifests)

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
	f.Start(manifests)

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
	f.b.nextBuildError = errors.New("failure")
	f.Start(manifests)

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	f.b.nextBuildError = errors.New("failure")
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
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
	f.b.nextBuildError = errors.New("failure")
	f.Start(manifests)

	// trigger a change for bar so that it would try to build if not for its resource dep
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("bar", "main.go"))

	call := f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.b.nextBuildError = errors.New("failure")
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("foo", "main2.go"))
	call = f.nextCall()
	require.Equal(t, "foo", call.local().Name.String())

	// since the foo build succeeded, bar should now queue
	call = f.nextCall()
	require.Equal(t, "bar", call.local().Name.String())
}

func TestLogsLongResourceName(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mn := strings.Repeat("foobar", 30)

	manifest := f.newManifest(mn)
	f.Start([]model.Manifest{manifest})

	call := f.nextCallComplete()
	assert.Equal(t, manifest.ImageTargetAt(0), call.firstImgTarg())
	assert.Equal(t, []string{}, call.oneImageState().FilesChanged())

	// this might be an annoying test since it depends on log formatting
	// its goal is to ensure we don't have dumb math that causes integer underflow or panics when it gets a long manifest name
	// thus, it just makes sure that we log that the manifest is building and we don't error,
	// and tries to limit how much it checks the formatting
	f.withState(func(state store.EngineState) {
		expectedLine := fmt.Sprintf("Initial Build • %s", mn)
		assert.Contains(t, state.LogStore.String(), expectedLine)
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerWontBuildManifestThatsAlreadyBuilding(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.b.completeBuildsManually = true

	// allow multiple builds at once; we care that we can't start multiple builds
	// of the same manifest, even if there ARE build slots available.
	f.setMaxParallelUpdates(3)

	manifest := f.newManifest("fe")
	f.Start([]model.Manifest{manifest})
	f.completeAndCheckBuildsForManifests(manifest)
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	f.waitUntilNumBuildSlots(3)

	// file change starts a build
	f.editFileAndWaitForManifestBuilding("fe", "A.txt")
	f.waitUntilNumBuildSlots(2)

	// a second file change doesn't start a second build, b/c 'fe' is already building
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("B.txt"))
	f.waitUntilNumBuildSlots(2) // still two build slots available

	// complete the first build
	f.completeBuildForManifest(manifest)

	call := f.nextCall("expect build from first pending file change (A.txt)")
	f.assertCallIsForManifestAndFiles(call, manifest, "A.txt")
	f.waitForCompletedBuildCount(2)
	f.podEvent(podbuilder.New(f.T(), manifest).Build(), manifest.Name)

	// we freed up a build slot; expect the second build to start
	f.waitUntilManifestBuilding("fe")

	f.completeBuildForManifest(manifest)
	call = f.nextCall("expect build from second pending file change (B.txt)")
	f.assertCallIsForManifestAndFiles(call, manifest, "B.txt")
	f.waitUntilManifestNotBuilding("fe")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerWontBuildManifestIfNoSlotsAvailable(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(2)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests (we only have 2 build slots)
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndAssertManifestNotBuilding("manC", "c/main.go")

	// Complete one build...
	f.completeBuildForManifest(manA)
	call := f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")

	// ...and now there's a free build slot for 'manC'
	f.waitUntilManifestBuilding("manC")

	// complete the rest (can't guarantee order)
	f.completeAndCheckBuildsForManifests(manB, manC)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

// It should be legal for a user to change maxParallelUpdates while builds
// are in progress (e.g. if there are 5 builds in progress and user sets
// maxParallelUpdates=3, nothing should explode.)
func TestCurrentlyBuildingMayExceedMaxParallelUpdates(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(3)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndWaitForManifestBuilding("manC", "c/main.go")
	f.waitUntilNumBuildSlots(0)

	// decrease maxParallelUpdates (now less than the number of current builds, but this is okay)
	f.setMaxParallelUpdates(2)
	f.waitUntilNumBuildSlots(0)

	// another file change for manB -- will try to start another build as soon as possible
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath("b/other.go"))

	f.completeBuildForManifest(manB)
	call := f.nextCall("expect manB build complete")
	f.assertCallIsForManifestAndFiles(call, manB, "b/main.go")

	// we should NOT see another build for manB, even though it has a pending file change,
	// b/c we don't have enough slots (since we decreased maxParallelUpdates)
	f.waitUntilNumBuildSlots(0)
	f.waitUntilManifestNotBuilding("manB")

	// complete another build...
	f.completeBuildForManifest(manA)
	call = f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")

	// ...now that we have an available slots again, manB will rebuild
	f.waitUntilManifestBuilding("manB")

	f.completeBuildForManifest(manB)
	call = f.nextCall("expect manB build complete (second build)")
	f.assertCallIsForManifestAndFiles(call, manB, "b/other.go")

	f.completeBuildForManifest(manC)
	call = f.nextCall("expect manC build complete")
	f.assertCallIsForManifestAndFiles(call, manC, "c/main.go")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestDontStartBuildIfControllerAndEngineUnsynced(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(3)

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	f.Start([]model.Manifest{manA, manB})
	f.completeAndCheckBuildsForManifests(manA, manB)

	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")

	// deliberately de-sync engine state and build controller
	st := f.store.LockMutableStateForTesting()
	st.StartedBuildCount--
	f.store.UnlockMutableState()

	// this build won't start while state and build controller are out of sync
	f.editFileAndAssertManifestNotBuilding("manB", "b/main.go")

	// resync the two counts...
	st = f.store.LockMutableStateForTesting()
	st.StartedBuildCount++
	f.store.UnlockMutableState()

	// ...and manB build will start as expected
	f.waitUntilManifestBuilding("manB")

	// complete all builds (can't guarantee order)
	f.completeAndCheckBuildsForManifests(manA, manB)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestErrorHandlingWithMultipleBuilds(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): fix this")
	}
	f := newTestFixture(t)
	defer f.TearDown()
	f.b.completeBuildsManually = true
	f.setMaxParallelUpdates(2)

	errA := fmt.Errorf("errA")
	errB := fmt.Errorf("errB")

	manA := f.newDockerBuildManifestWithBuildPath("manA", f.JoinPath("a"))
	manB := f.newDockerBuildManifestWithBuildPath("manB", f.JoinPath("b"))
	manC := f.newDockerBuildManifestWithBuildPath("manC", f.JoinPath("c"))
	f.Start([]model.Manifest{manA, manB, manC})
	f.completeAndCheckBuildsForManifests(manA, manB, manC)

	// start builds for all manifests (we only have 2 build slots)
	f.SetNextBuildError(errA)
	f.editFileAndWaitForManifestBuilding("manA", "a/main.go")
	f.SetNextBuildError(errB)
	f.editFileAndWaitForManifestBuilding("manB", "b/main.go")
	f.editFileAndAssertManifestNotBuilding("manC", "c/main.go")

	// Complete one build...
	f.completeBuildForManifest(manA)
	call := f.nextCall("expect manA build complete")
	f.assertCallIsForManifestAndFiles(call, manA, "a/main.go")
	f.WaitUntilManifestState("last manA build reflects expected error", "manA", func(ms store.ManifestState) bool {
		return ms.LastBuild().Error == errA
	})

	// ...'manC' should start building, even though the manA build ended with an error
	f.waitUntilManifestBuilding("manC")

	// complete the rest
	f.completeAndCheckBuildsForManifests(manB, manC)
	f.WaitUntilManifestState("last manB build reflects expected error", "manB", func(ms store.ManifestState) bool {
		return ms.LastBuild().Error == errB
	})
	f.WaitUntilManifestState("last manC build recorded and has no error", "manC", func(ms store.ManifestState) bool {
		return len(ms.BuildHistory) == 2 && ms.LastBuild().Error == nil
	})

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithSameTwoImages(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithSameTwoImages(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	aPath := f.JoinPath("common", "a.txt")
	f.fsWatcher.Events <- watch.NewFileEvent(aPath)

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	state := call.state[m1.ImageTargets[0].ID()]
	assert.Equal(t, map[string]bool{aPath: true}, state.FilesChangedSet)

	// Make sure that when the second build is triggered, we did the bookkeeping
	// correctly around marking the first and second image built and only deploying
	// the k8s resources.
	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	id := m2.ImageTargets[0].ID()
	result := f.b.resultsByID[id]
	assert.Equal(t, store.NewBuildState(result, nil, nil), call.state[id])

	id = m2.ImageTargets[1].ID()
	result = f.b.resultsByID[id]
	assert.Equal(t, store.NewBuildState(result, nil, nil), call.state[id])

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithTwoCommonAncestors(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithTwoCommonAncestors(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	aPath := f.JoinPath("base", "a.txt")
	f.fsWatcher.Events <- watch.NewFileEvent(aPath)

	f.waitForCompletedBuildCount(4)

	// Make sure that both builds are triggered, and that they
	// are triggered in a particular order.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	state := call.state[m1.ImageTargets[0].ID()]
	assert.Equal(t, map[string]bool{aPath: true}, state.FilesChangedSet)

	// Make sure that when the second build is triggered, we did the bookkeeping
	// correctly around marking the first and second image built, and only
	// rebuilding the third image and k8s deploy.
	call = f.nextCall("m2 build2")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	id := m2.ImageTargets[0].ID()
	result := f.b.resultsByID[id]
	assert.Equal(t,
		store.NewBuildState(result, nil, nil),
		call.state[id])

	id = m2.ImageTargets[1].ID()
	result = f.b.resultsByID[id]
	assert.Equal(t,
		store.NewBuildState(result, nil, nil),
		call.state[id])

	id = m2.ImageTargets[2].ID()
	result = f.b.resultsByID[id]

	// Assert the 3rd image was not reused from the previous build.
	assert.NotEqual(t, result, call.state[id].LastResult)
	assert.Equal(t,
		map[model.TargetID]bool{m2.ImageTargets[1].ID(): true},
		call.state[id].DepsChangedSet)

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestLocalDependsOnNonWorkloadK8s(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	local1 := manifestbuilder.New(f, "local").
		WithLocalResource("exec-local", nil).
		WithResourceDeps("k8s1").
		Build()
	k8s1 := manifestbuilder.New(f, "k8s1").
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodReadiness(model.PodReadinessIgnore).
		Build()
	f.Start([]model.Manifest{local1, k8s1})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("k8s1 build")
	assert.Equal(t, k8s1.K8sTarget(), call.k8s())

	call = f.nextCall("local build")
	assert.Equal(t, local1.LocalTarget(), call.local())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestManifestsWithCommonAncestorAndTrigger(t *testing.T) {
	f := newTestFixture(t)
	m1, m2 := NewManifestsWithCommonAncestor(f)
	f.Start([]model.Manifest{m1, m2})

	f.waitForCompletedBuildCount(2)

	call := f.nextCall("m1 build1")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	call = f.nextCall("m2 build1")
	assert.Equal(t, m2.K8sTarget(), call.k8s())

	f.store.Dispatch(server.AppendToTriggerQueueAction{Name: m1.Name})
	f.waitForCompletedBuildCount(3)

	// Make sure that only one build was triggered.
	call = f.nextCall("m1 build2")
	assert.Equal(t, m1.K8sTarget(), call.k8s())

	f.assertNoCall("m2 should not be rebuilt")

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func (f *testFixture) waitUntilManifestBuilding(name model.ManifestName) {
	msg := fmt.Sprintf("manifest %q is building", name)
	f.WaitUntilManifestState(msg, name, func(ms store.ManifestState) bool {
		return ms.IsBuilding()
	})

	f.withState(func(st store.EngineState) {
		_, ok := st.CurrentlyBuilding[name]
		require.True(f.t, ok, "expected EngineState to reflect that %q is currently building", name)
	})
}

func (f *testFixture) waitUntilManifestNotBuilding(name model.ManifestName) {
	msg := fmt.Sprintf("manifest %q is NOT building", name)
	f.WaitUntilManifestState(msg, name, func(ms store.ManifestState) bool {
		return !ms.IsBuilding()
	})

	f.withState(func(st store.EngineState) {
		_, ok := st.CurrentlyBuilding[name]
		require.False(f.t, ok, "expected EngineState to reflect that %q is NOT currently building", name)
	})
}

func (f *testFixture) waitUntilNumBuildSlots(expected int) {
	msg := fmt.Sprintf("%d build slots available", expected)
	f.WaitUntil(msg, func(st store.EngineState) bool {
		return expected == st.AvailableBuildSlots()
	})
}

func (f *testFixture) editFileAndWaitForManifestBuilding(name model.ManifestName, path string) {
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(path))
	f.waitUntilManifestBuilding(name)
}

func (f *testFixture) editFileAndAssertManifestNotBuilding(name model.ManifestName, path string) {
	f.fsWatcher.Events <- watch.NewFileEvent(f.JoinPath(path))
	f.waitUntilManifestNotBuilding(name)
}

func (f *testFixture) assertCallIsForManifestAndFiles(call buildAndDeployCall, m model.Manifest, files ...string) {
	assert.Equal(f.t, m.ImageTargetAt(0).ID(), call.firstImgTarg().ID())
	assert.Equal(f.t, f.JoinPaths(files), call.oneImageState().FilesChanged())
}

func (f *testFixture) completeAndCheckBuildsForManifests(manifests ...model.Manifest) {
	for _, m := range manifests {
		f.completeBuildForManifest(m)
	}

	expectedImageTargets := make([][]model.ImageTarget, len(manifests))
	var actualImageTargets [][]model.ImageTarget
	for i, m := range manifests {
		expectedImageTargets[i] = m.ImageTargets

		call := f.nextCall("timed out waiting for call %d/%d", i+1, len(manifests))
		actualImageTargets = append(actualImageTargets, call.imageTargets())
	}
	require.ElementsMatch(f.t, expectedImageTargets, actualImageTargets)

	for _, m := range manifests {
		f.waitUntilManifestNotBuilding(m.Name)
	}
}
