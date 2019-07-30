package containerupdate

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
)

var TestContainerInfo = store.ContainerInfo{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

func TestUpdateInContainerCopiesAndRmsFiles(t *testing.T) {
	f := newDCUFixture(t)

	archive := bytes.NewBuffer([]byte("hello world"))
	toDelete := []string{"/src/does-not-exist"}
	err := f.dcu.UpdateContainer(f.ctx, TestContainerInfo, archive, toDelete, nil, false)
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

func TestUpdateContainerExecsRuns(t *testing.T) {
	f := newDCUFixture(t)

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.dcu.UpdateContainer(f.ctx, TestContainerInfo, nil, nil, []model.Cmd{cmdA, cmdB}, false)
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []docker.ExecCall{
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdA},
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dCli.ExecCalls)
}

func TestUpdateContainerRestartsContainer(t *testing.T) {
	f := newDCUFixture(t)

	err := f.dcu.UpdateContainer(f.ctx, TestContainerInfo, nil, nil, nil, false)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dCli.RestartsByContainer[docker.TestContainer], 1)
}

func TestUpdateContainerHotReloadDoesNotRestartContainer(t *testing.T) {
	f := newDCUFixture(t)

	err := f.dcu.UpdateContainer(f.ctx, TestContainerInfo, nil, nil, nil, true)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, 0, len(f.dCli.RestartsByContainer))
}

func TestUpdateContainerKillTask(t *testing.T) {
	f := newDCUFixture(t)

	f.dCli.SetExecError(docker.ExitError{ExitCode: build.TaskKillExitCode})

	cmdA := model.Cmd{Argv: []string{"cat"}}
	err := f.dcu.UpdateContainer(f.ctx, TestContainerInfo, nil, nil, []model.Cmd{cmdA}, false)
	msg := "killed by container engine"
	if err == nil || !strings.Contains(err.Error(), msg) {
		f.t.Errorf("Expected error %q, actual: %v", msg, err)
	}

	expectedExecs := []docker.ExecCall{
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdA},
	}

	assert.Equal(f.t, expectedExecs, f.dCli.ExecCalls)
}

type dockerContainerUpdaterFixture struct {
	t    testing.TB
	ctx  context.Context
	dCli *docker.FakeClient
	dcu  *DockerContainerUpdater
}

func newDCUFixture(t testing.TB) *dockerContainerUpdaterFixture {
	fakeCli := docker.NewFakeClient()
	cu := &DockerContainerUpdater{dCli: fakeCli}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &dockerContainerUpdaterFixture{
		t:    t,
		ctx:  ctx,
		dCli: fakeCli,
		dcu:  cu,
	}
}
