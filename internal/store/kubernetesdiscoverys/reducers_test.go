package kubernetesdiscoverys

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
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
