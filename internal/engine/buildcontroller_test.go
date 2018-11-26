package engine

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

func TestBuildControllerOnePod(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, []string{}, call.state.FilesChanged())

	f.podEvent(f.testPod("pod-id", "fe", "Running", testContainer, time.Now()))
	f.fsWatcher.events <- watch.FileEvent{Path: "main.go"}

	call = f.nextCall()
	assert.Equal(t, "pod-id", call.state.DeployInfo.PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}

func TestBuildControllerTwoPods(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mount := model.Mount{LocalPath: "/go", ContainerPath: "/go"}
	manifest := f.newManifest("fe", []model.Mount{mount})
	f.Start([]model.Manifest{manifest}, true)

	call := f.nextCall()
	assert.Equal(t, manifest, call.manifest)
	assert.Equal(t, []string{}, call.state.FilesChanged())

	podA := f.testPod("pod-a", "fe", "Running", testContainer, time.Now())
	podB := f.testPod("pod-b", "fe", "Running", testContainer, time.Now())
	image := fmt.Sprintf("%s:%s", f.imageNameForManifest("fe").String(), "now")
	setImage(podA, image)
	setImage(podB, image)
	f.podEvent(podA)
	f.podEvent(podB)
	f.fsWatcher.events <- watch.FileEvent{Path: "main.go"}

	call = f.nextCall()
	assert.Equal(t, "", call.state.DeployInfo.PodID.String())

	err := f.Stop()
	assert.NoError(t, err)
	f.assertAllBuildsConsumed()
}
