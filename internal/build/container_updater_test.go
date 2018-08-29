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
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cu.containerIdForPod(f.ctx, testPod)
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), testContainer)
}

func TestContainerIdForPodFiltersOutPauseCmd(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	cID, err := f.cu.containerIdForPod(f.ctx, "one-pause-cmd")
	if err != nil {
		f.t.Fatal(err)
	}
	assert.Equal(f.t, cID.String(), "the right container")
}

func TestContainerIdForPodTooManyMatches(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cu.containerIdForPod(f.ctx, "too-many")
	if assert.NotNil(f.t, err) {
		assert.Contains(f.t, err.Error(), "too many matching containers")
	}
}

func TestContainerIdForPodNoNonPause(t *testing.T) {
	f := newRemoteDockerFixture(t)
	defer f.teardown()
	_, err := f.cu.containerIdForPod(f.ctx, "all-pause")
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
		assert.Equal(f.t, "/", f.dcli.CopyPath)
		assert.Equal(f.t, types.CopyToContainerOptions{}, f.dcli.CopyOptions)
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

// Integration test using a real docker client!
func TestUpdateInContainerE2E(t *testing.T) {
	f := newDockerBuildFixture(t)
	defer f.teardown()

	f.WriteFile("delete_me", "will be deleted")
	m := model.Mount{
		Repo:          model.LocalGithubRepo{LocalPath: f.Path()},
		ContainerPath: "/src",
	}

	// Allows us to track number of times the entrypoint has been called (i.e. how
	// many times container has been (re)started -- also, sleep forever so container
	// stays alive for us to manipulate.
	initStartcount := model.ToShellCmd("echo -n 0 > /src/startcount")
	entrypoint := model.ToShellCmd(
		"echo -n $(($(cat /src/startcount)+1)) > /src/startcount && sleep 1000000")

	imgRef, err := f.b.BuildImageFromScratch(f.ctx, f.getNameFromTest(), simpleDockerfile, []model.Mount{m}, []model.Cmd{initStartcount}, entrypoint)
	if err != nil {
		t.Fatal(err)
	}
	cID := f.startContainer(f.ctx, containerConfig(imgRef))

	f.Rm("delete_me") // expect to be delete from container on update
	f.WriteFile("foo", "hello world")

	paths := []pathMapping{
		pathMapping{LocalPath: f.JoinPath("delete_me"), ContainerPath: "/src/delete_me"},
		pathMapping{LocalPath: f.JoinPath("foo"), ContainerPath: "/src/foo"},
	}
	touchBar := model.ToShellCmd("touch /src/bar")

	cUpdater := containerUpdater{dcli: f.dcli}
	err = cUpdater.UpdateInContainer(f.ctx, cID, paths, []model.Cmd{touchBar})
	if err != nil {
		f.t.Fatal(err)
	}

	expected := []expectedFile{
		expectedFile{path: "/src/delete_me", missing: true},
		expectedFile{path: "/src/foo", contents: "hello world"},
		expectedFile{path: "/src/bar", contents: ""},         // from cmd
		expectedFile{path: "/src/startcount", contents: "2"}, // from entrypoint (confirm container restarted)
	}

	f.assertFilesInContainer(f.ctx, cID, expected)
}

type mockContainerUpdaterFixture struct {
	*testutils.TempDirFixture
	t    testing.TB
	ctx  context.Context
	dcli *FakeDockerClient
	cu   *containerUpdater
}

func newRemoteDockerFixture(t testing.TB) *mockContainerUpdaterFixture {
	fakeCli := NewFakeDockerClient()
	cu := &containerUpdater{
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
