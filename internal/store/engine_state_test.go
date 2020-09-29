package store

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestStateToViewRelativeEditPaths(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{}).WithImageTarget(model.ImageTarget{}.
		WithBuildDetails(model.DockerBuild{BuildPath: f.JoinPath("a", "b", "c")}))

	state := newState([]model.Manifest{m})
	ms := state.ManifestTargets[m.Name].State
	ms.CurrentBuild.Edits = []string{
		f.JoinPath("a", "b", "c", "foo"),
		f.JoinPath("a", "b", "c", "d", "e")}
	ms.BuildHistory = []model.BuildRecord{
		{
			Edits: []string{
				f.JoinPath("a", "b", "c", "foo"),
				f.JoinPath("a", "b", "c", "d", "e"),
			},
		},
	}
	ms.MutableBuildStatus(m.ImageTargets[0].ID()).PendingFileChanges =
		map[string]time.Time{
			f.JoinPath("a", "b", "c", "foo"):    time.Now(),
			f.JoinPath("a", "b", "c", "d", "e"): time.Now(),
		}
	v := StateToView(*state, &sync.RWMutex{})

	require.Len(t, v.Resources, 2)

	r, _ := v.Resource(m.Name)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.LastBuild().Edits)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.CurrentBuild.Edits)
	assert.Equal(t, []string{filepath.Join("d", "e"), "foo"}, r.PendingBuildEdits) // these are sorted for deterministic ordering
}

func TestStateToViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 7000, ContainerPort: 5001},
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"http://localhost:7000/", "http://localhost:8000/"},
		res.Endpoints)
}

func TestRuntimeStateNonWorkload(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	m := manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).
		Build()
	state := newState([]model.Manifest{m})
	runtimeState := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	assert.Equal(t, model.RuntimeStatusPending, runtimeState.RuntimeStatus())

	runtimeState.HasEverDeployedSuccessfully = true

	assert.Equal(t, model.RuntimeStatusOK, runtimeState.RuntimeStatus())
}

func TestStateToViewUnresourcedYAMLManifest(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"sancho:deployment"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func TestStateToViewNonWorkloadYAMLManifest(t *testing.T) {
	es, err := k8s.ParseYAMLFromString(testyaml.SecretYaml)
	require.NoError(t, err)
	m, err := k8s.NewK8sOnlyManifest(model.ManifestName("foo"), es, nil)
	require.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := StateToView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"mysecret:secret"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func TestMostRecentPod(t *testing.T) {
	podA := Pod{PodID: "pod-a", StartedAt: time.Now()}
	podB := Pod{PodID: "pod-b", StartedAt: time.Now().Add(time.Minute)}
	podC := Pod{PodID: "pod-c", StartedAt: time.Now().Add(-time.Minute)}
	m := model.Manifest{Name: "fe"}
	podSet := NewK8sRuntimeStateWithPods(m, podA, podB, podC)
	assert.Equal(t, "pod-b", podSet.MostRecentPod().PodID.String())
}

func TestRelativeTiltfilePath(t *testing.T) {
	es := newState([]model.Manifest{})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	es.TiltfilePath = filepath.Join(wd, "Tiltfile")

	actual, err := es.RelativeTiltfilePath()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Tiltfile", actual)
}

func TestNextBuildReason(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)

	kTarget := m.K8sTarget()
	mt := NewManifestTarget(m)

	iTargetID := model.ImageID(container.MustParseSelector("sancho"))
	status := mt.State.MutableBuildStatus(kTarget.ID())
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	status.PendingDependencyChanges[iTargetID] = time.Now()
	assert.Equal(t, "Initial Build",
		mt.NextBuildReason().String())

	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, "Dependency Updated",
		mt.NextBuildReason().String())

	status.PendingFileChanges["a.txt"] = time.Now()
	assert.Equal(t, "Changed Files | Dependency Updated",
		mt.NextBuildReason().String())
}
