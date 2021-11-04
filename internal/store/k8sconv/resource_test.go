package k8sconv

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
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

func TestNewKubernetesApplyFilter_Sorted(t *testing.T) {
	forDeploy, err := k8s.ParseYAMLFromString(testyaml.OutOfOrderYaml)
	require.NoError(t, err, "Invalid test YAML")
	for i := range forDeploy {
		forDeploy[i].SetUID(uuid.New().String())
	}
	resultYAML, err := k8s.SerializeSpecYAML(forDeploy)
	require.NoError(t, err, "Failed to re-serialize test YAML")
	// sanity check to ensure serialization isn't changing the sort
	require.Less(t, strings.Index(resultYAML, "Job"), strings.Index(resultYAML, "PersistentVolumeClaim"),
		"Order in re-serialized YAML was not preserved")

	applyFilter, err := NewKubernetesApplyFilter(&v1alpha1.KubernetesApplyStatus{
		ResultYAML: resultYAML,
	})
	require.NoError(t, err, "Failed to create KubernetesApplyFilter")
	require.NotNil(t, applyFilter, "KubernetesApplyFilter was nil")

	var actualKinds []string
	for _, ref := range applyFilter.DeployedRefs {
		actualKinds = append(actualKinds, ref.Kind)
	}

	expectedKindOrder := []string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap", "Service", "StatefulSet", "Job", "Pod"}
	assert.Equal(t, expectedKindOrder, actualKinds)
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
