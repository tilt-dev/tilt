package build

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestContainerIdForPodOneMatch(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, testPod)
	defer f.teardown()
	cID, err := f.b.containerIdForPod(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), testContainer)
}

func TestContainerIdForPodFiltersOutPauseCmd(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "one-pause-cmd")
	defer f.teardown()
	cID, err := f.b.containerIdForPod(context.Background())
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "the right container")
}

func TestContainerIdForPodTooManyMatches(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "too-many")
	defer f.teardown()
	_, err := f.b.containerIdForPod(context.Background())
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "too many matching containers")
	}
}

func TestContainerIdForPodNoNonPause(t *testing.T) {
	f := newRemoteDockerFixtureForPod(t, "all-pause")
	defer f.teardown()
	_, err := f.b.containerIdForPod(context.Background())
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "no matching non-'/pause' containers")
	}
}

func TestBuildDockerFromExistingCopiesAndRmsFiles(t *testing.T) {
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

	// TODO(maia): Check that we got a ref back i guess?
	_, err := f.b.BuildDockerFromExisting(f.ctx, nil, paths, []model.Cmd{})
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
		assert.Equal(f.t, "/", f.dcli.CopyPath)
		assert.Equal(f.t, types.CopyToContainerOptions{}, f.dcli.CopyOptions)
		// TODO(maia): assert that the right stuff made it into the archive (f.dcli.CopyContent)
	}
}

func TestBuildDockerFromExistingExecsSteps(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()

	cmdA := model.Cmd{Argv: []string{"a"}}
	cmdB := model.Cmd{Argv: []string{"b", "and b", "another b"}}

	_, err := f.b.BuildDockerFromExisting(f.ctx, nil, []pathMapping{}, []model.Cmd{cmdA, cmdB})
	if err != nil {
		f.t.Fatal(err)
	}

	expectedExecs := []ExecCall{
		ExecCall{Container: testContainer, Cmd: cmdA},
		ExecCall{Container: testContainer, Cmd: cmdB},
	}

	assert.Equal(f.t, expectedExecs, f.dcli.ExecCalls)
}

type remoteDockerFixture struct {
	*testutils.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dcli *FakeDockerClient
	b    *remoteDockerBuilder
}

func newRemoteDockerFixture(t testing.TB) *remoteDockerFixture {
	return newRemoteDockerFixtureForPod(t, testPod)
}

func newRemoteDockerFixtureForPod(t testing.TB, podName string) *remoteDockerFixture {
	fakeCli := NewFakeDockerClient()
	builder := &remoteDockerBuilder{
		dcli: fakeCli,
		pod:  podName,
	}

	return &remoteDockerFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            testutils.CtxForTest(),
		dcli:           fakeCli,
		b:              builder,
	}
}

func (f *remoteDockerFixture) teardown() {
	f.TempDirFixture.TearDown()
}
