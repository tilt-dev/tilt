package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestBuildControllerOnePod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
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

func TestBuildControllerWontContainerBuildWithTwoPods(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
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

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
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
		assert.Equal(t, model.BuildReasonFlagMountFiles, ms.LastBuild().Reason)
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

	mount := model.Mount{LocalPath: f.Path(), ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Init(InitAction{
		Manifests:       []model.Manifest{manifest},
		WatchMounts:     true,
		TriggerMode:     model.TriggerManual,
		ExecuteTiltfile: true,
	})

	f.nextCall()
	f.waitForCompletedBuildCount(1)

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "fe"})
	f.fsWatcher.events <- watch.FileEvent{Path: f.JoinPath("main.go")}

	f.WaitUntil("pending change appears", func(st store.EngineState) bool {
		return len(st.BuildStatus(manifest.ImageTargetAt(0).ID()).PendingFileChanges) > 0
	})

	// We don't expect a call because the trigger happened before the file event
	// came in.
	f.assertNoCall()

	f.store.Dispatch(view.AppendToTriggerQueueAction{Name: "fe"})
	call := f.nextCall()
	assert.Equal(t, []string{f.JoinPath("main.go")}, call.oneState().FilesChanged())
	f.waitForCompletedBuildCount(2)
}
