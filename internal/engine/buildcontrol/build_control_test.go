package buildcontrol

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
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

func (f *testFixture) upsertK8sManifest(name model.ManifestName) *store.ManifestTarget {
	return f.upsertManifest(manifestbuilder.New(f, name).
		WithK8sYAML(testyaml.SanchoYAML).
		Build())
}

func (f *testFixture) upsertLocalManifest(name model.ManifestName) *store.ManifestTarget {
	return f.upsertManifest(manifestbuilder.New(f, name).
		WithLocalResource(fmt.Sprintf("exec-%s", name), nil).
		Build())
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
