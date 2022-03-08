package buildcontrol

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/core/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var rsf = build.NewRunStepFailure(
	containerupdate.NewExecError(model.ToUnixCmd("omgwtfbbq"), 123),
)

var TestContainer = liveupdates.Container{
	PodID:         "somepod",
	ContainerID:   docker.TestContainer,
	ContainerName: "my-container",
	Namespace:     "ns-foo",
}

var TestContainers = []liveupdates.Container{TestContainer}

func TestBuildAndDeployBoilsSteps(t *testing.T) {
	f := newFixture(t)

	packageJson := build.PathMapping{LocalPath: f.JoinPath("package.json"), ContainerPath: "/src/package.json"}
	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Spec: v1alpha1.LiveUpdateSpec{
			BasePath: f.Path(),
			Execs: []v1alpha1.LiveUpdateExec{
				{Args: model.ToUnixCmd("./foo.sh bar").Argv},
				{Args: model.ToUnixCmd("yarn install").Argv, TriggerPaths: []string{"package.json"}},
				{Args: model.ToUnixCmd("pip install").Argv, TriggerPaths: []string{"requirements.txt"}},
			},
		},
		Input: liveupdate.Input{
			Containers:   TestContainers,
			ChangedFiles: []build.PathMapping{packageJson},
		},
	})
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

	// Write files so we know whether to cp to or rm from container
	f.WriteFile("hi", "hello")
	f.WriteFile("planets/earth", "world")

	paths := []build.PathMapping{
		build.PathMapping{LocalPath: f.JoinPath("hi"), ContainerPath: "/src/hi"},
		build.PathMapping{LocalPath: f.JoinPath("planets/earth"), ContainerPath: "/src/planets/earth"},
		build.PathMapping{LocalPath: f.JoinPath("does-not-exist"), ContainerPath: "/src/does-not-exist"},
	}

	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers:   TestContainers,
			ChangedFiles: paths,
		},
	})
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

	f.cu.SetUpdateErr(build.NewRunStepFailure(errors.New("oh no")))

	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers: TestContainers,
		},
	})
	if assert.NotNil(t, err) {
		assert.IsType(t, DontFallBackError{}, err)
	}
}

func TestUpdateContainerWithHotReload(t *testing.T) {
	f := newFixture(t)

	expectedHotReloads := []bool{true, true, false, true}
	for _, hotReload := range expectedHotReloads {
		restart := v1alpha1.LiveUpdateRestartStrategyNone
		if !hotReload {
			restart = v1alpha1.LiveUpdateRestartStrategyAlways
		}
		err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
			Spec: v1alpha1.LiveUpdateSpec{
				Restart: restart,
			},
			Input: liveupdate.Input{
				Containers: TestContainers,
			},
		})
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

	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers:   containers,
			ChangedFiles: paths,
		},
		Spec: v1alpha1.LiveUpdateSpec{
			Execs: []v1alpha1.LiveUpdateExec{{Args: cmd.Argv}},
		},
	})
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
	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers: containers,
		},
	})
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "👀")
	require.Len(t, f.cu.Calls, 1, "should only call UpdateContainer once (error should stop subsequent calls)")
}

func TestUpdateMultipleContainersWithSameTarArchive(t *testing.T) {
	f := newFixture(t)

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

	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers:   containers,
			ChangedFiles: paths,
		},
	})
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
	err := f.buildAndDeploy(f.ctx, f.ps, LiveUpdateInput{
		Input: liveupdate.Input{
			Containers:   containers,
			ChangedFiles: paths,
		},
	})
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "command \"omgwtfbbq\" failed with exit code: 123")

	require.Len(t, f.cu.Calls, 2)

	for i, call := range f.cu.Calls {
		assert.Equal(t, containers[i], call.ContainerInfo, "ContainerUpdater call[%d]", i)
		testutils.AssertFilesInTar(f.t, tar.NewReader(call.Archive), expected, "ContainerUpdater call[%d]", i)
	}
}

func TestSkipLiveUpdateIfForceUpdate(t *testing.T) {
	f := newFixture(t)

	m := manifestbuilder.New(f, "sancho").
		WithK8sYAML(SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(NewSanchoLiveUpdateImageTarget(f)).
		Build()

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
	t          testing.TB
	ctx        context.Context
	st         *store.TestingStore
	cu         *containerupdate.FakeContainerUpdater
	ps         *build.PipelineState
	lubad      *LiveUpdateBuildAndDeployer
	ctrlClient ctrlclient.Client
}

func newFixture(t testing.TB) *lcbadFixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	cu := &containerupdate.FakeContainerUpdater{}
	st := store.NewTestingStore()
	luReconciler := liveupdate.NewFakeReconciler(st, cu, cfb.Client)
	lubad := NewLiveUpdateBuildAndDeployer(luReconciler, fakeClock{})
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	return &lcbadFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		st:             st,
		ctx:            ctx,
		cu:             cu,
		ps:             build.NewPipelineState(ctx, 1, lubad.clock),
		lubad:          lubad,
		ctrlClient:     cfb.Client,
	}
}

func (f *lcbadFixture) buildAndDeploy(ctx context.Context, ps *build.PipelineState, input LiveUpdateInput) error {
	if input.Name == "" {
		input.Name = "fake-name"
	}

	lu := v1alpha1.LiveUpdate{
		ObjectMeta: metav1.ObjectMeta{Name: input.Name},
		Spec:       input.Spec,
	}
	f.upsert(&lu)
	return f.lubad.buildAndDeploy(ctx, ps, input)
}

func (f *lcbadFixture) upsert(obj ctrlclient.Object) {
	err := f.ctrlClient.Create(f.ctx, obj)
	if err == nil {
		return
	}

	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err = f.ctrlClient.Get(f.ctx, ktypes.NamespacedName{Name: obj.GetName()}, copy)
	assert.NoError(f.T(), err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = f.ctrlClient.Update(f.ctx, obj)
	assert.NoError(f.T(), err)
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
