package containerupdate

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

var TestDeployInfo = store.DeployInfo{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

func TestUpdateInContainerCopiesAndRmsFiles(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
		build.PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	err := f.cu.UpdateInContainer(f.ctx, TestDeployInfo, paths, model.EmptyMatcher, nil, false)
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(f.t, 1, len(f.dCli.ExecCalls), "calls to ExecInContainer") {
		assert.Equal(f.t, docker.TestContainer, f.dCli.ExecCalls[0].Container)
		expectedCmd := model.Cmd{Argv: []string{"rm", "-rf", "/src/does-not-exist"}}
		assert.Equal(f.t, expectedCmd, f.dCli.ExecCalls[0].Cmd)
	}

	if assert.Equal(f.t, 1, f.dCli.CopyCount, "calls to CopyToContainer") {
		assert.Equal(f.t, docker.TestContainer, f.dCli.CopyContainer)
		// TODO(maia): assert that the right stuff made it into the archive (f.dCli.CopyContent)
	}
}

func TestUpdateInContainerExecsRuns(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.cu.UpdateInContainer(f.ctx, TestDeployInfo, []build.PathMapping{}, model.EmptyMatcher, []model.Cmd{cmdA, cmdB}, false)
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []docker.ExecCall{
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdA},
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dCli.ExecCalls)
}

func TestUpdateInContainerRestartsContainer(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	err := f.cu.UpdateInContainer(f.ctx, TestDeployInfo, []build.PathMapping{}, model.EmptyMatcher, nil, false)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dCli.RestartsByContainer[docker.TestContainer], 1)
}

func TestUpdateInContainerHotReloadDoesNotRestartContainer(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	err := f.cu.UpdateInContainer(f.ctx, TestDeployInfo, []build.PathMapping{}, model.EmptyMatcher, nil, true)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, 0, len(f.dCli.RestartsByContainer))
}

func TestUpdateInContainerKillTask(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	f.dCli.ExecErrorToThrow = docker.ExitError{ExitCode: build.TaskKillExitCode}

	cmdA := model.Cmd{Argv: []string{"cat"}}
	err := f.cu.UpdateInContainer(f.ctx, TestDeployInfo, []build.PathMapping{}, model.EmptyMatcher, []model.Cmd{cmdA}, false)
	msg := "killed by container engine"
	if err == nil || !strings.Contains(err.Error(), msg) {
		f.t.Errorf("Expected error %q, actual: %v", msg, err)
	}

	expectedExecs := []docker.ExecCall{
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdA},
	}

	assert.Equal(f.t, expectedExecs, f.dCli.ExecCalls)
}

type mockContainerUpdaterFixture struct {
	*tempdir.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dCli *docker.FakeClient
	cu   *DockerContainerUpdater
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := docker.NewFakeClient()
	cu := &DockerContainerUpdater{
		dCli: fakeCli,
	}

	return &mockContainerUpdaterFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            output.CtxForTest(),
		dCli:           fakeCli,
		cu:             cu,
	}
}

func (f *mockContainerUpdaterFixture) teardown() {
	f.TempDirFixture.TearDown()
}
