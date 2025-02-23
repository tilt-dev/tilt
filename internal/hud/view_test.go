package hud

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/hud/view"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestStateToViewRelativeEditPaths(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{}).WithImageTarget(model.ImageTarget{}.
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.JoinPath("a", "b", "c")}))

	state := newState([]model.Manifest{m})
	ms := state.ManifestTargets[m.Name].State
	ms.CurrentBuilds["buildcontrol"] = model.BuildRecord{
		Edits: []string{
			f.JoinPath("a", "b", "c", "foo"),
			f.JoinPath("a", "b", "c", "d", "e"),
		},
	}
	ms.BuildHistory = []model.BuildRecord{
		{
			Edits: []string{
				f.JoinPath("a", "b", "c", "foo"),
				f.JoinPath("a", "b", "c", "d", "e"),
			},
		},
	}
	ms.MutableBuildStatus(m.ImageTargets[0].ID()).FileChanges =
		map[string]time.Time{
			f.JoinPath("a", "b", "c", "foo"):    time.Now(),
			f.JoinPath("a", "b", "c", "d", "e"): time.Now(),
		}
	v := StateToTerminalView(*state, &sync.RWMutex{})

	require.Len(t, v.Resources, 2)

	r, _ := v.Resource(m.Name)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.LastBuild().Edits)
	assert.Equal(t, []string{"foo", filepath.Join("d", "e")}, r.CurrentBuild.Edits)
	assert.Equal(t, []string{filepath.Join("d", "e"), "foo"}, r.PendingBuildEdits) // these are sorted for deterministic ordering
}

func TestStateToTerminalViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					{LocalPort: 8000, ContainerPort: 5000},
					{LocalPort: 7000, ContainerPort: 5001},
				},
			},
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToTerminalView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"http://localhost:8000/", "http://localhost:7000/"},
		res.Endpoints)
}

func TestStateToWebViewLinksAndPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					{LocalPort: 8000, ContainerPort: 5000},
					{LocalPort: 8001, ContainerPort: 5001, Name: "debugger"},
				},
			},
		},
		Links: []model.Link{
			model.MustNewLink("www.apple.edu", "apple"),
			model.MustNewLink("www.zombo.com", "zombo"),
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToTerminalView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"www.apple.edu", "www.zombo.com", "http://localhost:8000/", "http://localhost:8001/"},
		res.Endpoints)
}
func TestStateToTerminalViewLocalResourceLinks(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.LocalTarget{
		Links: []model.Link{
			model.MustNewLink("www.apple.edu", "apple"),
			model.MustNewLink("www.zombo.com", "zombo"),
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToTerminalView(*state, &sync.RWMutex{})
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"www.apple.edu", "www.zombo.com"},
		res.Endpoints)
}

func TestRuntimeStateNonWorkload(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)

	m := manifestbuilder.New(f, model.UnresourcedYAMLManifestName).
		WithK8sYAML(testyaml.SecretYaml).
		Build()
	state := newState([]model.Manifest{m})
	runtimeState := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	assert.Equal(t, v1alpha1.RuntimeStatusPending, runtimeState.RuntimeStatus())

	runtimeState.HasEverDeployedSuccessfully = true

	assert.Equal(t, v1alpha1.RuntimeStatusOK, runtimeState.RuntimeStatus())
}

func TestRuntimeStateJob(t *testing.T) {
	for _, tc := range []struct {
		phase                 v1.PodPhase
		expectedRuntimeStatus v1alpha1.RuntimeStatus
	}{
		{v1.PodRunning, v1alpha1.RuntimeStatusPending},
		{v1.PodSucceeded, v1alpha1.RuntimeStatusOK},
		{v1.PodFailed, v1alpha1.RuntimeStatusError},
	} {
		t.Run(string(tc.phase), func(t *testing.T) {
			f := tempdir.NewTempDirFixture(t)

			m := manifestbuilder.New(f, "foo").
				WithK8sYAML(testyaml.JobYAML).
				WithK8sPodReadiness(model.PodReadinessSucceeded).
				Build()
			state := newState([]model.Manifest{m})
			runtimeState := state.ManifestTargets[m.Name].State.K8sRuntimeState()
			assert.Equal(t, v1alpha1.RuntimeStatusPending, runtimeState.RuntimeStatus())

			runtimeState.HasEverDeployedSuccessfully = true

			pod := v1alpha1.Pod{
				Name:      "pod",
				CreatedAt: apis.Now(),
				Phase:     string(tc.phase),
			}
			runtimeState.FilteredPods = []v1alpha1.Pod{pod}

			assert.Equal(t, tc.expectedRuntimeStatus, runtimeState.RuntimeStatus())
		})
	}
}

func TestRuntimeStateJobCompleteMissingPods(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)

	m := manifestbuilder.New(f, "foo").
		WithK8sYAML(testyaml.JobYAML).
		WithK8sPodReadiness(model.PodReadinessSucceeded).
		Build()
	state := newState([]model.Manifest{m})
	runtimeState := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	runtimeState.HasEverDeployedSuccessfully = true
	assert.Equal(t, v1alpha1.RuntimeStatusPending, runtimeState.RuntimeStatus())

	// N.B. there are no pods but a condition attached
	runtimeState.Conditions = []metav1.Condition{
		{
			Type:   v1alpha1.ApplyConditionJobComplete,
			Status: metav1.ConditionTrue,
		},
	}
	assert.Equal(t, v1alpha1.RuntimeStatusOK, runtimeState.RuntimeStatus())
}

func TestStateToTerminalViewUnresourcedYAMLManifest(t *testing.T) {
	m := k8sManifest(t, model.UnresourcedYAMLManifestName, testyaml.SanchoYAML)
	state := newState([]model.Manifest{m})
	krs := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	krs.ApplyFilter = yamlToApplyFilter(t, testyaml.SanchoYAML)
	state.ManifestTargets[m.Name].State.RuntimeState = krs

	v := StateToTerminalView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"sancho:deployment"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func TestStateToTerminalViewNonWorkloadYAMLManifest(t *testing.T) {
	m := k8sManifest(t, "foo", testyaml.SecretYaml)
	state := newState([]model.Manifest{m})
	krs := state.ManifestTargets[m.Name].State.K8sRuntimeState()
	krs.ApplyFilter = yamlToApplyFilter(t, testyaml.SecretYaml)
	state.ManifestTargets[m.Name].State.RuntimeState = krs

	v := StateToTerminalView(*state, &sync.RWMutex{})

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := view.YAMLResourceInfo{
		K8sDisplayNames: []string{"mysecret:secret"},
	}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
}

func newState(manifests []model.Manifest) *store.EngineState {
	ret := store.NewState()
	for _, m := range manifests {
		ret.ManifestTargets[m.Name] = store.NewManifestTarget(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}

	return ret
}

func yamlToApplyFilter(t testing.TB, yaml string) *k8sconv.KubernetesApplyFilter {
	t.Helper()
	entities, err := k8s.ParseYAMLFromString(yaml)
	require.NoError(t, err, "Failed to parse YAML")
	for i := range entities {
		entities[i].SetUID(uuid.New().String())
	}
	yaml, err = k8s.SerializeSpecYAML(entities)
	require.NoError(t, err, "Failed to re-serialize YAML")
	applyFilter, err := k8sconv.NewKubernetesApplyFilter(yaml)
	require.NoError(t, err, "Failed to create KubernetesApplyFilter")
	require.NotNil(t, applyFilter, "ApplyFilter was nil")
	return applyFilter
}

func k8sManifest(t testing.TB, name model.ManifestName, yaml string) model.Manifest {
	t.Helper()
	kt, err := k8s.NewTargetForYAML(name.TargetName(), yaml, nil)
	require.NoError(t, err, "Failed to create Kubernetes deploy target")
	return model.Manifest{Name: name}.WithDeployTarget(kt)
}
