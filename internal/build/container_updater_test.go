package build

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestUpdateInContainerCopiesAndRmsFiles(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []PathMapping{
		PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
		PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, paths, model.EmptyMatcher, nil, ioutil.Discard)
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

func TestUpdateInContainerExecsSteps(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, []PathMapping{}, model.EmptyMatcher, []model.Cmd{cmdA, cmdB}, ioutil.Discard)
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

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, []PathMapping{}, model.EmptyMatcher, nil, ioutil.Discard)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dCli.RestartsByContainer[docker.TestContainer], 1)
}

type mockContainerUpdaterFixture struct {
	*tempdir.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dCli *docker.FakeClient
	cu   *ContainerUpdater
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := docker.NewFakeClient()
	cu := &ContainerUpdater{
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
