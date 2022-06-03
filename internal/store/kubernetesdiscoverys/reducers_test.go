package kubernetesdiscoverys

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestActionDeleteApplyFirst(t *testing.T) {
	ka := &v1alpha1.KubernetesApply{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	kd := &v1alpha1.KubernetesDiscovery{ObjectMeta: metav1.ObjectMeta{Name: "a"}}

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
	ka := &v1alpha1.KubernetesApply{ObjectMeta: metav1.ObjectMeta{Name: "a"}}
	kd := &v1alpha1.KubernetesDiscovery{ObjectMeta: metav1.ObjectMeta{Name: "a"}}

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

func TestNewKubernetesApplyFilterOnlyUsesYaml(t *testing.T) {
	// RefreshKubernetesResource assumes that KubernetesApplyFilters are pure functions of the ResultYAML,
	// and reuses filters if the yaml hasn't changed, to save time on re-parsing. If we change the signature
	// of NewKubernetesApplyFilter to depend on anything else, then we need to change how RefreshKubernetesResource
	// decides whether to reuse a resource's filter.
	// This test exists purely to draw a dev's attention to the above comment if they change NewKubernetesApplyFilter's signature.
	_, _ = k8sconv.NewKubernetesApplyFilter("")
}
