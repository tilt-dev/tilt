package build

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestContainerIdForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cu.ContainerIDForPod(f.ctx, testPod)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), testContainer)
}

func TestContainerIdForPodFiltersOutPauseCmd(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cu.ContainerIDForPod(f.ctx, "one-pause-cmd")
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "the right container")
}

func TestContainerIdForPodTooManyMatches(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cu.ContainerIDForPod(f.ctx, "too-many")
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "too many matching containers")
	}
}

func TestContainerIdForPodNoNonPause(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cu.ContainerIDForPod(f.ctx, "all-pause")
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "no matching non-'/pause' containers")
	}
}

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

	err := f.cu.UpdateInContainer(f.ctx, testContainer, paths, []model.Cmd{})
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(f.t, 1, len(f.dcli.ExecCalls), "calls to ExecInContainer") {
		assert.Equal(f.t, testContainer, f.dcli.ExecCalls[0].Container)
		expectedCmd := model.Cmd{Argv: []string{"rm", "-rf", "/src/does-not-exist"}}
		assert.Equal(f.t, expectedCmd, f.dcli.ExecCalls[0].Cmd)
	}

	if assert.Equal(f.t, 1, f.dcli.CopyCount, "calls to CopyToContainer") {
		assert.Equal(f.t, testContainer, f.dcli.CopyContainer)
		// TODO(maia): assert that the right stuff made it into the archive (f.dcli.CopyContent)
	}
}

func TestUpdateInContainerExecsSteps(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.cu.UpdateInContainer(f.ctx, testContainer, []pathMapping{}, []model.Cmd{cmdA, cmdB})
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []ExecCall{
		ExecCall{Container: testContainer, Cmd: cmdA},
		ExecCall{Container: testContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dcli.ExecCalls)
}

func TestUpdateInContainerRestartsContainer(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	err := f.cu.UpdateInContainer(f.ctx, testContainer, []pathMapping{}, []model.Cmd{})
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dcli.RestartsByContainer[testContainer], 1)
}

type mockContainerUpdaterFixture struct {
	*testutils.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dcli *FakeDockerClient
	cu   *ContainerUpdater
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := NewFakeDockerClient()
	cu := &ContainerUpdater{
		dcli: fakeCli,
	}

	return &mockContainerUpdaterFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            testutils.CtxForTest(),
		dcli:           fakeCli,
		cu:             cu,
	}
}

func (f *mockContainerUpdaterFixture) teardown() {
	f.TempDirFixture.TearDown()
}
