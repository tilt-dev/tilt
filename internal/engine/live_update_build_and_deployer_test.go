package engine

import (
	"archive/tar"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

var rsf = build.RunStepFailure{
	Cmd:      model.ToShellCmd("omgwtfbbq"),
	ExitCode: 123,
}

var TestContainerInfo = store.ContainerInfo{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

var TestBuildState = store.BuildState{
	LastSuccessfulResult: alreadyBuilt,
	FilesChangedSet:      map[string]bool{"foo.py": true},
	RunningContainers:    []store.ContainerInfo{TestContainerInfo},
}

func TestBuildAndDeployBoilsSteps(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	packageJson := build.PathMapping{LocalPath: f.JoinPath("package.json"), ContainerPath: "/src/package.json"}
	runs := []model.Run{
		model.ToRun(model.ToShellCmd("./foo.sh bar")),
		model.Run{Cmd: model.ToShellCmd("yarn install"), Triggers: f.newPathSet("package.json")},
		model.Run{Cmd: model.ToShellCmd("pip install"), Triggers: f.newPathSet("requirements.txt")},
	}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestBuildState, []build.PathMapping{packageJson}, runs, false)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Len(t, f.cu.Calls, 1) {
		t.FailNow()
	}

	call := f.cu.Calls[0]
	expectedCmds := []model.Cmd{
		model.ToShellCmd("./foo.sh bar"), // should always run
		model.ToShellCmd("yarn install"), // should run b/c we changed `package.json`
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

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestBuildState, paths, nil, false)
	if err != nil {
		t.Fatal(err)
	}

	if !assert.Len(t, f.cu.Calls, 1) {
		t.FailNow()
	}

	call := f.cu.Calls[0]
	expectedToDelete := []string{"/src/does-not-exist"}
	assert.Equal(t, expectedToDelete, call.ToDelete)

	expected := []expectedFile{
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

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestBuildState, nil, nil, false)
	if assert.NotNil(t, err) {
		assert.IsType(t, buildcontrol.DontFallBackError{}, err)
	}
}

func TestUpdateContainerWithHotReload(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	expectedHotReloads := []bool{true, true, false, true}
	for _, hotReload := range expectedHotReloads {
		err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, TestBuildState, nil, nil, hotReload)
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

	cInfo1 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	cInfo2 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	cInfos := []store.ContainerInfo{cInfo1, cInfo2}
	state := store.BuildState{
		LastSuccessfulResult: alreadyBuilt,
		FilesChangedSet:      map[string]bool{"foo.py": true},
		RunningContainers:    cInfos,
	}

	paths := []build.PathMapping{
		// Will try to delete this file
		build.PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	cmd := model.ToShellCmd("./foo.sh bar")
	runs := []model.Run{model.ToRun(cmd)}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, state, paths, runs, true)
	if err != nil {
		t.Fatal(err)
	}

	expectedToDelete := []string{"/src/does-not-exist"}

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, cInfos[i], call.ContainerInfo)
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

	cInfo1 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	cInfo2 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	cInfos := []store.ContainerInfo{cInfo1, cInfo2}
	state := store.BuildState{
		LastSuccessfulResult: alreadyBuilt,
		FilesChangedSet:      map[string]bool{"foo.py": true},
		RunningContainers:    cInfos,
	}

	f.cu.SetUpdateErr(fmt.Errorf("👀"))
	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, state, nil, nil, false)
	require.NotNil(t, err)
	assert.Contains(t, "👀", err.Error())
	require.Len(t, f.cu.Calls, 1, "should only call UpdateContainer once (error should stop subsequent calls)")
}

func TestUpdateMultipleContainersWithSameTarArchive(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cInfo1 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	cInfo2 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	cInfos := []store.ContainerInfo{cInfo1, cInfo2}
	state := store.BuildState{
		LastSuccessfulResult: alreadyBuilt,
		FilesChangedSet:      map[string]bool{"foo.py": true},
		RunningContainers:    cInfos,
	}

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
	}
	expected := []expectedFile{
		expectFile("src/hi", "hello"),
		expectFile("src/planets/earth", "world"),
	}

	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, state, paths, nil, true)
	if err != nil {
		t.Fatal(err)
	}

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, cInfos[i], call.ContainerInfo)
		testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected)
	}
}

func TestUpdateMultipleContainersWithSameTarArchiveOnRunStepFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cInfo1 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}
	cInfo2 := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid2",
		ContainerName: "container2",
		Namespace:     "ns-foo",
	}

	cInfos := []store.ContainerInfo{cInfo1, cInfo2}
	state := store.BuildState{
		LastSuccessfulResult: alreadyBuilt,
		FilesChangedSet:      map[string]bool{"foo.py": true},
		RunningContainers:    cInfos,
	}

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
	}
	expected := []expectedFile{
		expectFile("src/hi", "hello"),
		expectFile("src/planets/earth", "world"),
	}

	f.cu.UpdateErrs = []error{rsf, rsf}
	err := f.lubad.buildAndDeploy(f.ctx, f.ps, f.cu, model.ImageTarget{}, state, paths, nil, true)
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "Run step \"omgwtfbbq\" failed with exit code: 123")

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, cInfos[i], call.ContainerInfo, "ContainerUpdater call[%d]", i)
		testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected, "ContainerUpdater call[%d]", i)
	}
}

func TestSkipLiveUpdateIfForceUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	m := NewSanchoLiveUpdateManifest(f)

	cInfo := store.ContainerInfo{
		PodID:         "mypod",
		ContainerID:   "cid1",
		ContainerName: "container1",
		Namespace:     "ns-foo",
	}

	state := store.BuildState{
		LastSuccessfulResult: alreadyBuilt,
		RunningContainers:    []store.ContainerInfo{cInfo},
		ImageBuildTriggered:  true, // should make us skip LiveUpdate
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
	st    *store.Store
	cu    *containerupdate.FakeContainerUpdater
	ps    *build.PipelineState
	lubad *LiveUpdateBuildAndDeployer
}

func newFixture(t testing.TB) *lcbadFixture {
	// HACK(maia): we don't need any real container updaters on this LiveUpdBaD since we're testing
	// a func further down the flow that takes a ContainerUpdater as an arg, so just pass nils
	lubad := NewLiveUpdateBuildAndDeployer(nil, nil, nil, buildcontrol.UpdateModeAuto, k8s.EnvDockerDesktop, container.RuntimeDocker, fakeClock{})
	fakeContainerUpdater := &containerupdate.FakeContainerUpdater{}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreForTesting()
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

func expectFile(path, contents string) expectedFile {
	return testutils.ExpectedFile{
		Path:     path,
		Contents: contents,
		Missing:  false,
	}
}

func expectMissing(path string) expectedFile {
	return testutils.ExpectedFile{
		Path:    path,
		Missing: true,
	}
}
