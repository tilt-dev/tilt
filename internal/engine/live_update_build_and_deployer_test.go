package engine

import (
	"archive/tar"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/containerupdate"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

var TestDeployInfo = store.DeployInfo{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

var TestBuildState = store.BuildState{
	LastResult:      alreadyBuilt,
	FilesChangedSet: map[string]bool{"foo.py": true},
	DeployInfo:      TestDeployInfo,
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

	err := f.lubad.buildAndDeploy(f.ctx, f.cu, model.ImageTarget{}, TestBuildState, []build.PathMapping{packageJson}, runs, false)
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

	err := f.lubad.buildAndDeploy(f.ctx, f.cu, model.ImageTarget{}, TestBuildState, paths, nil, false)
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

	f.cu.UpdateErr = build.UserBuildFailure{ExitCode: 12345}

	err := f.lubad.buildAndDeploy(f.ctx, f.cu, model.ImageTarget{}, TestBuildState, nil, nil, false)
	if assert.NotNil(t, err) {
		assert.IsType(t, DontFallBackError{}, err)
	}
}

func TestUpdateContainerWithHotReload(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	expectedHotReloads := []bool{true, true, false, true}
	for _, hotReload := range expectedHotReloads {
		err := f.lubad.buildAndDeploy(f.ctx, f.cu, model.ImageTarget{}, TestBuildState, nil, nil, hotReload)
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

type lcbadFixture struct {
	*tempdir.TempDirFixture
	t     testing.TB
	ctx   context.Context
	st    *store.Store
	cu    *containerupdate.FakeContainerUpdater
	lubad *LiveUpdateBuildAndDeployer
}

func newFixture(t testing.TB) *lcbadFixture {
	// HACK(maia): we don't need any real container updaters on this LiveUpdBaD since we're testing
	// a func further down the flow that takes a ContainerUpdater as an arg, so just pass nils
	lubad := NewLiveUpdateBuildAndDeployer(nil, nil, nil, UpdateModeAuto, k8s.EnvDockerDesktop, container.RuntimeDocker)
	fakeContainerUpdater := &containerupdate.FakeContainerUpdater{}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreForTesting()
	return &lcbadFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		st:             st,
		ctx:            ctx,
		cu:             fakeContainerUpdater,
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
