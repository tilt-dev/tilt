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

	paths := []pathMapping{
		pathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		pathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
		pathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, paths, model.EmptyMatcher, nil, ioutil.Discard)
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(f.t, 1, len(f.dcli.ExecCalls), "calls to ExecInContainer") {
		assert.Equal(f.t, docker.TestContainer, f.dcli.ExecCalls[0].Container)
		expectedCmd := model.Cmd{Argv: []string{"rm", "-rf", "/src/does-not-exist"}}
		assert.Equal(f.t, expectedCmd, f.dcli.ExecCalls[0].Cmd)
	}

	if assert.Equal(f.t, 1, f.dcli.CopyCount, "calls to CopyToContainer") {
		assert.Equal(f.t, docker.TestContainer, f.dcli.CopyContainer)
		// TODO(maia): assert that the right stuff made it into the archive (f.dcli.CopyContent)
	}
}

func TestUpdateInContainerExecsSteps(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, []pathMapping{}, model.EmptyMatcher, []model.Cmd{cmdA, cmdB}, ioutil.Discard)
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []docker.ExecCall{
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdA},
		docker.ExecCall{Container: docker.TestContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dcli.ExecCalls)
}

func TestUpdateInContainerRestartsContainer(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	err := f.cu.UpdateInContainer(f.ctx, docker.TestContainer, []pathMapping{}, model.EmptyMatcher, nil, ioutil.Discard)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dcli.RestartsByContainer[docker.TestContainer], 1)
}

type mockContainerUpdaterFixture struct {
	*tempdir.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dcli *docker.FakeDockerClient
	cu   *ContainerUpdater
	cr   *ContainerResolver
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := docker.NewFakeDockerClient()
	cu := &ContainerUpdater{
		dcli: fakeCli,
	}
	cr := &ContainerResolver{
		dcli: fakeCli,
	}

	return &mockContainerUpdaterFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            output.CtxForTest(),
		dcli:           fakeCli,
		cu:             cu,
		cr:             cr,
	}
}

func (f *mockContainerUpdaterFixture) teardown() {
	f.TempDirFixture.TearDown()
}
