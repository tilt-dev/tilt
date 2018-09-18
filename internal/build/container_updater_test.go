package build

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/wmdocker"
)

func TestContainerIdForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cu.ContainerIDForPod(f.ctx, wmdocker.TestPod)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), wmdocker.TestContainer)
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

	err := f.cu.UpdateInContainer(f.ctx, wmdocker.TestContainer, paths, nil)
	if err != nil {
		f.t.Fatal(err)
	}

	if assert.Equal(f.t, 1, len(f.dcli.ExecCalls), "calls to ExecInContainer") {
		assert.Equal(f.t, wmdocker.TestContainer, f.dcli.ExecCalls[0].Container)
		expectedCmd := model.Cmd{Argv: []string{"rm", "-rf", "/src/does-not-exist"}}
		assert.Equal(f.t, expectedCmd, f.dcli.ExecCalls[0].Cmd)
	}

	if assert.Equal(f.t, 1, f.dcli.CopyCount, "calls to CopyToContainer") {
		assert.Equal(f.t, wmdocker.TestContainer, f.dcli.CopyContainer)
		// TODO(maia): assert that the right stuff made it into the archive (f.dcli.CopyContent)
	}
}

func TestUpdateInContainerExecsSteps(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"cu", "and cu", "another cu"}}

	err := f.cu.UpdateInContainer(f.ctx, wmdocker.TestContainer, []pathMapping{}, []model.Cmd{cmdA, cmdB})
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []wmdocker.ExecCall{
		wmdocker.ExecCall{Container: wmdocker.TestContainer, Cmd: cmdA},
		wmdocker.ExecCall{Container: wmdocker.TestContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dcli.ExecCalls)
}

func TestUpdateInContainerRestartsContainer(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	err := f.cu.UpdateInContainer(f.ctx, wmdocker.TestContainer, []pathMapping{}, nil)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, f.dcli.RestartsByContainer[wmdocker.TestContainer], 1)
}

type mockContainerUpdaterFixture struct {
	*tempdir.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dcli *wmdocker.FakeDockerClient
	cu   *ContainerUpdater
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := wmdocker.NewFakeDockerClient()
	cu := &ContainerUpdater{
		dcli: fakeCli,
	}

	return &mockContainerUpdaterFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            output.CtxForTest(),
		dcli:           fakeCli,
		cu:             cu,
	}
}

func (f *mockContainerUpdaterFixture) teardown() {
	f.TempDirFixture.TearDown()
}
