package kubernetesdiscoverys

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestActionDeleteApplyFirst(t *testing.T) {
	ka := newApply("a")
	kd := newDiscovery("a", nil)

	state := store.NewState()
	state.KubernetesApplys[ka.Name] = ka
	HandleKubernetesDiscoveryUpsertAction(state, KubernetesDiscoveryUpsertAction{
		KubernetesDiscovery: kd,
	})
	assert.Equal(t, kd, state.KubernetesResources[kd.Name].Discovery)

	delete(state.KubernetesApplys, ka.Name)
	HandleKubernetesDiscoveryDeleteAction(state, KubernetesDiscoveryDeleteAction{
		Name: kd.Name,
	})
	assert.Nil(t, state.KubernetesResources[kd.Name].Discovery)
}

func TestActionDeleteDiscoFirst(t *testing.T) {
	ka := newApply("a")
	kd := newDiscovery("a", nil)

	state := store.NewState()
	state.KubernetesApplys[ka.Name] = ka
	HandleKubernetesDiscoveryUpsertAction(state, KubernetesDiscoveryUpsertAction{
		KubernetesDiscovery: kd,
	})
	assert.Equal(t, kd, state.KubernetesResources[kd.Name].Discovery)

	HandleKubernetesDiscoveryDeleteAction(state, KubernetesDiscoveryDeleteAction{
		Name: kd.Name,
	})
	assert.Nil(t, state.KubernetesResources[kd.Name].Discovery)
}

func TestReadyJob(t *testing.T) {
	start := metav1.Now()
	podA := v1alpha1.Pod{
		Name:      "pod-a",
		Namespace: "default",
		Phase:     string(v1.PodFailed),
		CreatedAt: metav1.NewTime(start.Add(-time.Hour)),
	}
	podB := v1alpha1.Pod{
		Name:      "pod-b",
		Namespace: "default",
		Phase:     string(v1.PodSucceeded),
		CreatedAt: start,
	}

	ka := newApply("a")
	kd := newDiscovery("a", []v1alpha1.Pod{podA, podB})

	state := store.NewState()
	mt := store.NewManifestTarget(model.Manifest{Name: "a"})
	state.UpsertManifestTarget(mt)
	state.KubernetesApplys[ka.Name] = ka

	ms, ok := state.ManifestState("a")
	require.True(t, ok)
	krs := ms.K8sRuntimeState()
	krs.HasEverDeployedSuccessfully = true
	ms.RuntimeState = krs
	assert.False(t, krs.HasEverBeenReadyOrSucceeded())

	HandleKubernetesDiscoveryUpsertAction(state, KubernetesDiscoveryUpsertAction{
		KubernetesDiscovery: kd,
	})
	assert.True(t, ms.K8sRuntimeState().HasEverBeenReadyOrSucceeded())
}

func TestNewKubernetesApplyFilterOnlyUsesYaml(t *testing.T) {
	// RefreshKubernetesResource assumes that KubernetesApplyFilters are pure functions of the ResultYAML,
	// and reuses filters if the yaml hasn't changed, to save time on re-parsing. If we change the signature
	// of NewKubernetesApplyFilter to depend on anything else, then we need to change how RefreshKubernetesResource
	// decides whether to reuse a resource's filter.
	// This test exists purely to draw a dev's attention to the above comment if they change NewKubernetesApplyFilter's signature.
	_, _ = k8sconv.NewKubernetesApplyFilter("")
}

func newApply(name string) *v1alpha1.KubernetesApply {
	return &v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: name,
			},
		},
	}
}

func newDiscovery(name string, pods []v1alpha1.Pod) *v1alpha1.KubernetesDiscovery {
	return &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: name,
			},
		},
		Status: v1alpha1.KubernetesDiscoveryStatus{
			Pods: pods,
		},
	}
}
