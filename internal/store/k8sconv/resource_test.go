package k8sconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestFilteredPodByAncestorUID(t *testing.T) {
	applyStatus := newDeploymentApplyStatus()

	podA := v1alpha1.Pod{Name: "pod-1", Namespace: "default"}
	podB := v1alpha1.Pod{Name: "pod-1", Namespace: "default", AncestorUID: "328372c6-a93a-460a-9bc7-eff90c69f5ce"}
	podC := v1alpha1.Pod{Name: "pod-1", Namespace: "default", AncestorUID: "328372c6-a93a-460a-9bc7-cab"}
	discovery := newDiscovery([]v1alpha1.Pod{podA, podB, podC})
	res, err := NewKubernetesResource(discovery, applyStatus)
	require.NoError(t, err)
	assert.Equal(t, []v1alpha1.Pod{podA, podB}, res.FilteredPods)
}

func TestFilteredPodByPodTemplateHash(t *testing.T) {
	applyStatus := newDeploymentApplyStatus()

	podA := v1alpha1.Pod{Name: "pod-1", Namespace: "default"}
	podB := v1alpha1.Pod{Name: "pod-1", Namespace: "default", PodTemplateSpecHash: "9e6121753dfe0fbb65ed"}
	podC := v1alpha1.Pod{Name: "pod-1", Namespace: "default", PodTemplateSpecHash: "328372c6-a93a-460a-9bc7-cab"}
	discovery := newDiscovery([]v1alpha1.Pod{podA, podB, podC})
	res, err := NewKubernetesResource(discovery, applyStatus)
	require.NoError(t, err)
	assert.Equal(t, []v1alpha1.Pod{podA, podB}, res.FilteredPods)
}

func newDeploymentApplyStatus() *v1alpha1.KubernetesApplyStatus {
	return &v1alpha1.KubernetesApplyStatus{
		ResultYAML: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tilt-site
  namespace: default
  resourceVersion: "41313"
  uid: 328372c6-a93a-460a-9bc7-eff90c69f5ce
spec:
  selector:
    matchLabels:
      app: tilt-site
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: tilt-site
        app.kubernetes.io/managed-by: tilt
        tilt.dev/pod-template-hash: 9e6121753dfe0fbb65ed
    spec:
      containers:
      - image: localhost:5005/tilt-site:tilt-bb6b20cd3041242e
        name: tilt-site
`,
	}
}

func newDiscovery(pods []v1alpha1.Pod) *v1alpha1.KubernetesDiscovery {
	return &v1alpha1.KubernetesDiscovery{
		Status: v1alpha1.KubernetesDiscoveryStatus{
			Pods: pods,
		},
	}
}
