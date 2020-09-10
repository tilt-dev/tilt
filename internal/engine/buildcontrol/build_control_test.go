package buildcontrol

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestNextTargetToBuildDoesntReturnCurrentlyBuildingTarget(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	mt := f.manifestNeedingCrashRebuild()
	f.st.UpsertManifestTarget(mt)

	// Verify this target is normally next-to-build
	f.assertNextTargetToBuild(mt.Manifest.Name)

	// If target is currently building, should NOT be next-to-build
	mt.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestCurrentlyBuildingK8sResourceDisablesLocalScheduling(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8s1 := f.upsertK8sManifest("k8s1")
	k8s2 := f.upsertK8sManifest("k8s2")
	f.upsertLocalManifest("local1")

	f.assertNextTargetToBuild("local1")

	k8s1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s2")

	k8s2.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestCurrentlyBuildingUncategorizedDisablesOtherK8sTargets(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	_ = f.upsertK8sManifest("k8s1")
	k8sUnresourced := f.upsertK8sManifest(model.UnresourcedYAMLManifestName)
	_ = f.upsertK8sManifest("k8s2")

	f.assertNextTargetToBuild(model.UnresourcedYAMLManifestName)
	k8sUnresourced.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestK8sDependsOnLocal(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	k8s1 := f.upsertK8sManifest("k8s1", withResourceDeps("local1"))
	k8s2 := f.upsertK8sManifest("k8s2")
	local1 := f.upsertLocalManifest("local1")

	f.assertNextTargetToBuild("local1")

	local1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	local1.State.RuntimeState = store.LocalRuntimeState{HasSucceededAtLeastOnce: true}

	f.assertNextTargetToBuild("k8s1")
	k8s1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s2")

	_ = k8s2
}

func TestLocalDependsOnNonWorkloadK8s(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	local1 := f.upsertLocalManifest("local1", withResourceDeps("k8s1"))
	k8s1 := f.upsertK8sManifest("k8s1", withK8sPodReadiness(model.PodReadinessIgnore))
	k8s2 := f.upsertK8sManifest("k8s2", withK8sPodReadiness(model.PodReadinessIgnore))

	f.assertNextTargetToBuild("k8s1")

	k8s1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	k8s1.State.RuntimeState = store.K8sRuntimeState{
		PodReadinessMode:            model.PodReadinessIgnore,
		HasEverDeployedSuccessfully: true,
	}

	f.assertNextTargetToBuild("local1")
	local1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	f.assertNextTargetToBuild("k8s2")

	_ = k8s2
}

func TestCurrentlyBuildingLocalResourceDisablesK8sScheduling(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.upsertK8sManifest("k8s1")
	local1 := f.upsertLocalManifest("local1")
	f.upsertLocalManifest("local2")

	f.assertNextTargetToBuild("local1")

	local1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNoTargetNextToBuild()
}

func TestCurrentlyBuildingParallelLocalResource(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	f.upsertK8sManifest("k8s1")
	local1 := f.upsertLocalManifest("local1", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithLocalAllowParallel(true)
	})
	local2 := f.upsertLocalManifest("local2", func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithLocalAllowParallel(true)
	})

	f.assertNextTargetToBuild("local1")

	local1.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("local2")

	local2.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	f.assertNextTargetToBuild("k8s1")
}

func TestTwoK8sTargetsWithBaseImage(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	baseImage := model.MustNewImageTarget(container.MustParseSelector("sancho-base"))
	sanchoOneImage := model.MustNewImageTarget(container.MustParseSelector("sancho-one")).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})
	sanchoTwoImage := model.MustNewImageTarget(container.MustParseSelector("sancho-two")).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})

	sanchoOne := f.upsertManifest(manifestbuilder.New(f, "sancho-one").
		WithImageTargets(baseImage, sanchoOneImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())
	f.upsertManifest(manifestbuilder.New(f, "sancho-two").
		WithImageTargets(baseImage, sanchoTwoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())

	f.assertNextTargetToBuild("sancho-one")

	sanchoOne.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}

	f.assertNoTargetNextToBuild()
	sanchoOne.State.CurrentBuild = model.BuildRecord{}
	sanchoOne.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})

	f.assertNextTargetToBuild("sancho-two")
}

func TestTwoK8sTargetsWithBaseImagePrebuilt(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	baseImage := model.MustNewImageTarget(container.MustParseSelector("sancho-base"))
	sanchoOneImage := model.MustNewImageTarget(container.MustParseSelector("sancho-one")).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})
	sanchoTwoImage := model.MustNewImageTarget(container.MustParseSelector("sancho-two")).
		WithDependencyIDs([]model.TargetID{baseImage.ID()})

	sanchoOne := f.upsertManifest(manifestbuilder.New(f, "sancho-one").
		WithImageTargets(baseImage, sanchoOneImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())
	sanchoTwo := f.upsertManifest(manifestbuilder.New(f, "sancho-two").
		WithImageTargets(baseImage, sanchoTwoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())

	sanchoOne.State.MutableBuildStatus(baseImage.ID()).LastResult = store.ImageBuildResult{}
	sanchoTwo.State.MutableBuildStatus(baseImage.ID()).LastResult = store.ImageBuildResult{}

	f.assertNextTargetToBuild("sancho-one")

	sanchoOne.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}

	// Make sure sancho-two can start while sanchoOne is still pending.
	f.assertNextTargetToBuild("sancho-two")
}

func TestHoldForDeploy(t *testing.T) {
	f := newTestFixture(t)
	defer f.TearDown()

	srcFile := f.JoinPath("src", "a.txt")
	objFile := f.JoinPath("obj", "a.out")
	fallbackFile := f.JoinPath("src", "package.json")
	f.WriteFile(srcFile, "hello")
	f.WriteFile(objFile, "hello")
	f.WriteFile(fallbackFile, "hello")

	lu, err := model.NewLiveUpdate([]model.LiveUpdateStep{
		model.LiveUpdateFallBackOnStep{Files: []string{f.JoinPath("src", "package.json")}},
		model.LiveUpdateSyncStep{Source: f.JoinPath("src"), Dest: "/src"},
	}, f.Path())
	require.NoError(t, err)

	sanchoImage := model.MustNewImageTarget(container.MustParseSelector("sancho")).
		WithBuildDetails(model.DockerBuild{BuildPath: f.Path()})
	sancho := f.upsertManifest(manifestbuilder.New(f, "sancho").
		WithImageTargets(sanchoImage).
		WithLiveUpdate(lu).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())

	f.assertNextTargetToBuild("sancho")

	sancho.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	f.assertNoTargetNextToBuild()

	status := sancho.State.MutableBuildStatus(sanchoImage.ID())

	status.PendingFileChanges[objFile] = time.Now()
	f.assertNextTargetToBuild("sancho")
	delete(status.PendingFileChanges, objFile)

	status.PendingFileChanges[fallbackFile] = time.Now()
	f.assertNextTargetToBuild("sancho")
	delete(status.PendingFileChanges, fallbackFile)

	status.PendingFileChanges[srcFile] = time.Now()
	f.assertNoTargetNextToBuild()
	f.assertHold("sancho", store.HoldWaitingForDeploy)

	sancho.State.K8sRuntimeState().Pods["pod-1"] = successPod("pod-1", sanchoImage.Refs.ClusterRef())
	f.assertNextTargetToBuild("sancho")
}

func successPod(podID k8s.PodID, ref reference.Named) *store.Pod {
	return &store.Pod{
		PodID:  podID,
		Phase:  v1.PodSucceeded,
		Status: "Running",
		Containers: []store.Container{
			store.Container{
				ID:       container.ID(podID + "-container"),
				Name:     "c",
				Ready:    true,
				Running:  true,
				ImageRef: ref,
			},
		},
	}
}

type testFixture struct {
	*tempdir.TempDirFixture
	t  *testing.T
	st *store.EngineState
}

func newTestFixture(t *testing.T) testFixture {
	f := tempdir.NewTempDirFixture(t)
	return testFixture{
		TempDirFixture: f,
		t:              t,
		st:             store.NewState(),
	}
}

func (f *testFixture) assertHold(m model.ManifestName, hold store.Hold) {
	f.T().Helper()
	_, hs := NextTargetToBuild(*f.st)
	assert.Equal(f.t, hold, hs[m])
}

func (f *testFixture) assertNextTargetToBuild(expected model.ManifestName) {
	f.T().Helper()
	next, _ := NextTargetToBuild(*f.st)
	require.NotNil(f.t, next, "expected next target %s but got: nil", expected)
	actual := next.Manifest.Name
	assert.Equal(f.t, expected, actual, "expected next target to be %s but got %s", expected, actual)
}

func (f *testFixture) assertNoTargetNextToBuild() {
	f.T().Helper()
	next, _ := NextTargetToBuild(*f.st)
	if next != nil {
		f.t.Fatalf("expected no next target to build, but got %s", next.Manifest.Name)
	}
}

func (f *testFixture) upsertManifest(m model.Manifest) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	f.st.UpsertManifestTarget(mt)
	return mt
}

func (f *testFixture) upsertK8sManifest(name model.ManifestName, opts ...manifestOption) *store.ManifestTarget {
	b := manifestbuilder.New(f, name)
	for _, o := range opts {
		b = o(b)
	}
	return f.upsertManifest(b.WithK8sYAML(testyaml.SanchoYAML).Build())
}

func (f *testFixture) upsertLocalManifest(name model.ManifestName, opts ...manifestOption) *store.ManifestTarget {
	b := manifestbuilder.New(f, name)
	for _, o := range opts {
		b = o(b)
	}
	return f.upsertManifest(b.WithLocalResource(fmt.Sprintf("exec-%s", name), nil).Build())
}

func (f *testFixture) manifestNeedingCrashRebuild() *store.ManifestTarget {
	m := manifestbuilder.New(f, "needs-crash-rebuild").
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		model.BuildRecord{
			StartTime:  time.Now().Add(-5 * time.Second),
			FinishTime: time.Now(),
		},
	}
	mt.State.NeedsRebuildFromCrash = true
	return mt
}

type manifestOption func(manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder

func withResourceDeps(deps ...string) manifestOption {
	return manifestOption(func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithResourceDeps(deps...)
	})
}
func withK8sPodReadiness(pr model.PodReadinessMode) manifestOption {
	return manifestOption(func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithK8sPodReadiness(pr)
	})
}
