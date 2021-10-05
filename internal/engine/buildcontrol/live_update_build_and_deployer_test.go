package buildcontrol

import (
	"archive/tar"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

var rsf = build.RunStepFailure{
	Cmd:      model.ToUnixCmd("omgwtfbbq"),
	ExitCode: 123,
}

var TestContainer = liveupdates.Container{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

var TestContainers = []liveupdates.Container{TestContainer}

func TestBuildAndDeployBoilsSteps(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	packageJson := build.PathMapping{LocalPath: f.JoinPath("package.json"), ContainerPath: "/src/package.json"}
	runs := []model.Run{
		model.ToRun(model.ToUnixCmd("./foo.sh bar")),
		model.Run{Cmd: model.ToUnixCmd("yarn install"), Triggers: f.newPathSet("package.json")},
		model.Run{Cmd: model.ToUnixCmd("pip install"), Triggers: f.newPathSet("requirements.txt")},
	}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestContainers, []build.PathMapping{packageJson}, runs, false)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Len(t, f.cu.Calls, 1) {
		t.FailNow()
	}

	call := f.cu.Calls[0]
	expectedCmds := []model.Cmd{
		model.ToUnixCmd("./foo.sh bar"), // should always run
		model.ToUnixCmd("yarn install"), // should run b/c we changed `package.json`
		// `pip install` should NOT run b/c we didn't change `requirements.txt`
	}
	assert.Equal(t, expectedCmds, call.Cmds)
}

func TestUpdateInContainerArchivesFilesToCopyAndGetsFilesToRemove(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
		build.PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestContainers, paths, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Len(t, f.cu.Calls, 1) {
		t.FailNow()
	}

	call := f.cu.Calls[0]
	expectedToDelete := []string{"/src/does-not-exist"}
	assert.Equal(t, expectedToDelete, call.ToDelete)

	expected := []testutils.ExpectedFile{
		expectFile("src/hi", "hello"),
		expectFile("src/planets/earth", "world"),
		expectMissing("src/does-not-exist"),
	}
	testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected)
}

func TestDontFallBackOnUserError(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.cu.SetUpdateErr(build.RunStepFailure{ExitCode: 12345})

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestContainers, nil, nil, false)
	if assert.NotNil(t, err) {
		assert.IsType(t, DontFallBackError{}, err)
	}
}

func TestUpdateContainerWithHotReload(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	expectedHotReloads := []bool{true, true, false, true}
	for _, hotReload := range expectedHotReloads {
		err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestContainers, nil, nil, hotReload)
		if err != nil {
			t.Fatal(err)
		}
	}

	if assert.Len(t, f.cu.Calls, len(expectedHotReloads)) {
		for i, call := range f.cu.Calls {
			assert.Equal(t, expectedHotReloads[i], call.HotReload,
				"expected f.cu.Calls[%d] to have HotReload = %t", i, expectedHotReloads[i])
		}
	}
}

func TestUpdateMultipleRunningContainers(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	container1 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	container2 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	containers := []liveupdates.Container{container1, container2}

	paths := []build.PathMapping{
		// Will try to delete this file
		build.PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	cmd := model.ToUnixCmd("./foo.sh bar")
	runs := []model.Run{model.ToRun(cmd)}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, containers, paths, runs, true)
	if err != nil {
		t.Fatal(err)
	}

	expectedToDelete := []string{"/src/does-not-exist"}

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, containers[i], call.ContainerInfo)
		assert.Equal(t, expectedToDelete, call.ToDelete)
		if assert.Len(t, call.Cmds, 1) {
			assert.Equal(t, cmd, call.Cmds[0])
		}
		assert.True(t, call.HotReload)
	}
}

func TestErrorStopsSubsequentContainerUpdates(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	container1 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	container2 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	containers := []liveupdates.Container{container1, container2}

	f.cu.SetUpdateErr(fmt.Errorf("👀"))
	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, containers, nil, nil, false)
	require.NotNil(t, err)
	assert.Contains(t, "👀", err.Error())
	require.Len(t, f.cu.Calls, 1, "should only call UpdateContainer once (error should stop subsequent calls)")
}

func TestUpdateMultipleContainersWithSameTarArchive(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	container1 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	container2 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	containers := []liveupdates.Container{container1, container2}

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
	}
	expected := []testutils.ExpectedFile{
		expectFile("src/hi", "hello"),
		expectFile("src/planets/earth", "world"),
	}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, containers, paths, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, containers[i], call.ContainerInfo)
		testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected)
	}
}

func TestUpdateMultipleContainersWithSameTarArchiveOnRunStepFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	container1 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	container2 := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	containers := []liveupdates.Container{container1, container2}

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
	}
	expected := []testutils.ExpectedFile{
		expectFile("src/hi", "hello"),
		expectFile("src/planets/earth", "world"),
	}

	f.cu.UpdateErrs = []error{rsf, rsf}
	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, containers, paths, nil, true)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Run step \"omgwtfbbq\" failed with exit code: 123")

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, containers[i], call.ContainerInfo, "ContainerUpdater call[%d]", i)
		testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected, "ContainerUpdater call[%d]", i)
	}
}

func TestSkipLiveUpdateIfForceUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	m := NewSanchoLiveUpdateManifest(f)

	container := liveupdates.Container{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}

	imageName := string(m.ImageTargetAt(0).ID().Name)
	state := store.BuildState{
		LastResult:         alreadyBuilt,
		KubernetesResource: liveupdates.FakeKubernetesResource(imageName, []liveupdates.Container{container}),
		FullBuildTriggered: true, // should make us skip LiveUpdate
	}

	stateSet := store.BuildStateSet{m.ImageTargetAt(0).ID(): state}

	_, err := f.lubad.BuildAndDeploy(f.ctx, f.st, m.TargetSpecs(), stateSet)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Force update", "expected error contents not found")
}

type lcbadFixture struct {
	*tempdir.TempDirFixture
	t     testing.TB
	ctx   context.Context
	st    *store.TestingStore
	cu    *containerupdate.FakeContainerUpdater
	ps    *build.PipelineState
	lubad *LiveUpdateBuildAndDeployer
}

func newFixture(t testing.TB) *lcbadFixture {
	// HACK(maia): we don't need any real container updaters on this LiveUpdBaD since we're testing
	// a func further down the flow that takes a ContainerUpdater as an arg, so just pass nils
	lubad := NewLiveUpdateBuildAndDeployer(nil, nil, UpdateModeAuto, k8s.KubeContext("fake-context"), fakeClock{})
	fakeContainerUpdater := &containerupdate.FakeContainerUpdater{}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st := store.NewTestingStore()
	return &lcbadFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		st:             st,
		ctx:            ctx,
		cu:             fakeContainerUpdater,
		ps:             build.NewPipelineState(ctx, 1, lubad.clock),
		lubad:          lubad,
	}
}

func (f *lcbadFixture) teardown() {
	f.TempDirFixture.TearDown()
}

func (f *lcbadFixture) newPathSet(paths ...string) model.PathSet {
	return model.NewPathSet(paths, f.Path())
}

func expectFile(path, contents string) testutils.ExpectedFile {
	return testutils.ExpectedFile{
		Path:     path,
		Contents: contents,
		Missing:  false,
	}
}

func expectMissing(path string) testutils.ExpectedFile {
	return testutils.ExpectedFile{
		Path:    path,
		Missing: true,
	}
}
