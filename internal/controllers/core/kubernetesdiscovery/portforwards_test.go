package kubernetesdiscovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestPortForwardCreateAndUpdate(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)
	key := types.NamespacedName{Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Name: "kd"},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					v1alpha1.Forward{LocalPort: 4000, ContainerPort: 4000},
				},
			},
		},
	}

	f.Create(kd)
	f.kClient.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID})

	// Simulate the reconcile (which would normally be invoked by the manager on status update).
	f.MustReconcile(key)

	var pf v1alpha1.PortForward
	f.MustGet(types.NamespacedName{Name: "kd-pod"}, &pf)
	require.Equal(t, 1, len(pf.Spec.Forwards))
	assert.Equal(t, 4000, int(pf.Spec.Forwards[0].LocalPort))

	f.MustGet(key, kd)
	kd.Spec.PortForwardTemplateSpec.Forwards[0].LocalPort = 4001
	f.Update(kd)

	f.MustReconcile(key)

	f.MustGet(types.NamespacedName{Name: "kd-pod"}, &pf)
	require.Equal(t, 1, len(pf.Spec.Forwards))
	assert.Equal(t, 4001, int(pf.Spec.Forwards[0].LocalPort))
}

func TestPortForwardCreateAndDelete(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)
	key := types.NamespacedName{Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					v1alpha1.Forward{LocalPort: 4000, ContainerPort: 4000},
				},
			},
		},
	}

	f.Create(kd)
	f.kClient.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID})

	// Simulate the reconcile (which would normally be invoked by the manager on status update).
	f.MustReconcile(key)

	var pf v1alpha1.PortForward
	f.MustGet(types.NamespacedName{Name: "kd-pod"}, &pf)

	f.MustGet(key, kd)
	kd.Spec.PortForwardTemplateSpec = nil
	f.Update(kd)

	f.MustReconcile(key)
	assert.False(t, f.Get(types.NamespacedName{Name: "kd-pod"}, &pf))
}

func TestPortForwardCreateAndDeleteOwner(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)
	key := types.NamespacedName{Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					v1alpha1.Forward{LocalPort: 4000, ContainerPort: 4000},
				},
			},
		},
	}

	f.Create(kd)
	f.kClient.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID})

	// Simulate the reconcile (which would normally be invoked by the manager on status update).
	f.MustReconcile(key)

	var pf v1alpha1.PortForward
	assert.True(t, f.Get(types.NamespacedName{Name: "kd-pod"}, &pf))

	f.Delete(kd)

	f.MustReconcile(key)
	assert.False(t, f.Get(types.NamespacedName{Name: "kd-pod"}, &pf))
}
