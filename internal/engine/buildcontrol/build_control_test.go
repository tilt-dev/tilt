package buildcontrol

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
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
	k8s1 := f.upsertK8sManifest("k8s1", withK8sNonWorkload())
	k8s2 := f.upsertK8sManifest("k8s2", withK8sNonWorkload())

	f.assertNextTargetToBuild("k8s1")

	k8s1.State.AddCompletedBuild(model.BuildRecord{
		StartTime:  time.Now(),
		FinishTime: time.Now(),
	})
	k8s1.State.RuntimeState = store.K8sRuntimeState{NonWorkload: true, HasEverDeployedSuccessfully: true}

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

func (f *testFixture) assertNextTargetToBuild(expected model.ManifestName) {
	next := NextTargetToBuild(*f.st)
	require.NotNil(f.t, next, "expected next target %s but got: nil", expected)
	actual := next.Manifest.Name
	assert.Equal(f.t, expected, actual, "expected next target to be %s but got %s", expected, actual)
}

func (f *testFixture) assertNoTargetNextToBuild() {
	next := NextTargetToBuild(*f.st)
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
func withK8sNonWorkload() manifestOption {
	return manifestOption(func(m manifestbuilder.ManifestBuilder) manifestbuilder.ManifestBuilder {
		return m.WithK8sNonWorkload()
	})
}
