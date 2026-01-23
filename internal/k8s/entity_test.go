package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestTypedPodGVK(t *testing.T) {
	entity := NewK8sEntity(&v1.Pod{})
	assert.Equal(t, "", entity.GVK().Group)
	assert.Equal(t, "v1", entity.GVK().Version)
	assert.Equal(t, "Pod", entity.GVK().Kind)
}

func TestTypedDeploymentGVK(t *testing.T) {
	entity := NewK8sEntity(&appsv1.Deployment{})
	assert.Equal(t, "apps", entity.GVK().Group)
	assert.Equal(t, "v1", entity.GVK().Version)
	assert.Equal(t, "Deployment", entity.GVK().Kind)
}

func TestName(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(entities))
	assert.Equal(t, "devel-nick-lb-blorg-be", entities[0].Name())
	assert.Equal(t, "devel-nick-blorg-be", entities[1].Name())
}

func TestNamespace(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SyncletYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(entities))
	assert.Equal(t, "kube-system", string(entities[0].Namespace()))
}

func TestLoadBalancerSpecs(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	lbs := ToLoadBalancerSpecs(entities)
	if len(lbs) != 1 {
		t.Fatalf("Expected 1 loadbalancer, actual %d: %v", len(lbs), lbs)
	}

	if lbs[0].Name != "devel-nick-lb-blorg-be" ||
		lbs[0].Ports[0] != 8080 {
		t.Fatalf("Unexpected loadbalancer: %+v", lbs[0])
	}
}

func TestFilter(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.BlorgBackendYAML, testyaml.BlorgJobYAML)
	if err != nil {
		t.Fatal(err)
	}

	test := func(e K8sEntity) (bool, error) {
		if e.GVK().Kind == "Deployment" || e.GVK().Kind == "Job" {
			return true, nil
		}
		return false, nil
	}

	popped, rest, err := Filter(entities, test)
	if err != nil {
		t.Fatal(err)
	}
	expectedPopped := []K8sEntity{entities[1], entities[2]} // deployment, job
	expectedRest := []K8sEntity{entities[0]}                // service
	assert.Equal(t, popped, expectedPopped)
	assert.Equal(t, rest, expectedRest)

	returnFalse := func(e K8sEntity) (bool, error) { return false, nil }
	popped, rest, err = Filter(entities, returnFalse)
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, popped)
	assert.Equal(t, rest, entities)

	returnErr := func(e K8sEntity) (bool, error) {
		return false, fmt.Errorf("omgwtfbbq")
	}
	_, _, err = Filter(entities, returnErr)
	if assert.Error(t, err, "expected Filter to propagate err from test func") {
		assert.Equal(t, err.Error(), "omgwtfbbq")
	}
}

func TestHasName(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d: %v", len(entities), entities)
	}

	doggos := entities[0]
	assert.True(t, doggos.HasName(testyaml.DoggosName))

	snack := entities[1]
	assert.False(t, snack.HasName(testyaml.DoggosName))
}

func TestHasNamespace(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.SnackYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d: %v", len(entities), entities)
	}

	doggos := entities[0]
	assert.True(t, doggos.HasNamespace(testyaml.DoggosNamespace))

	snack := entities[1]
	assert.False(t, snack.HasNamespace(testyaml.DoggosNamespace))
}

func TestHasKind(t *testing.T) {
	entities, err := parseYAMLFromStrings(testyaml.DoggosDeploymentYaml, testyaml.DoggosServiceYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d: %v", len(entities), entities)
	}

	depl := entities[0]
	assert.True(t, depl.HasKind("deployment"))
	assert.False(t, depl.HasKind("service"))

	svc := entities[1]
	assert.False(t, svc.HasKind("deployment"))
	assert.True(t, svc.HasKind("service"))
}

func TestSortEntities(t *testing.T) {
	for _, test := range []struct {
		name              string
		inputKindOrder    []string
		expectedKindOrder []string
	}{
		{"all explicitly sorted",
			[]string{"Deployment", "Namespace", "Service"},
			[]string{"Namespace", "Service", "Deployment"},
		},
		{"preserve order if not explicitly sorted",
			[]string{"custom1", "custom2", "custom3"},
			[]string{"custom1", "custom2", "custom3"},
		},
		{"preserve order if not explicitly sorted, also sort others",
			[]string{"custom1", "custom2", "Secret", "custom3", "ConfigMap"},
			[]string{"ConfigMap", "Secret", "custom1", "custom2", "custom3"},
		},
		{"pod and job not sorted",
			[]string{"Pod", "Job", "Job", "Pod"},
			[]string{"Pod", "Job", "Job", "Pod"},
		},
		{"preserve order if not explicitly sorted if many elements",
			// sort.Sort started by comparing input[0] and input[6], which resulted in unexpected order.
			// (didn't preserve order of "Job" vs. "Pod"). Make sure that doesn't happen anymore.
			[]string{"Job", "PersistentVolumeClaim", "Service", "Pod", "ConfigMap", "PersistentVolume", "StatefulSet"},
			[]string{"PersistentVolume", "PersistentVolumeClaim", "ConfigMap", "Service", "StatefulSet", "Job", "Pod"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := entitiesWithKinds(test.inputKindOrder)
			sorted := SortedEntities(input)
			assertKindOrder(t, test.expectedKindOrder, sorted, "sorted entities")
		})
	}
}

func TestClean(t *testing.T) {
	yaml := `apiVersion: v1
kind: Namespace
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"Namespace","metadata":{"annotations":{},"labels":{"app.kubernetes.io/managed-by":"tilt"},"name":"elastic-system"},"spec":{}}
  creationTimestamp: "2020-07-07T14:50:17Z"
  labels:
    app.kubernetes.io/managed-by: tilt
  managedFields:
  - apiVersion: v1
    fieldsType: FieldsV1
    fieldsV1:
      f:metadata:
        f:annotations:
          .: {}
          f:kubectl.kubernetes.io/last-applied-configuration: {}
        f:labels:
          .: {}
          f:app.kubernetes.io/managed-by: {}
      f:status:
        f:phase: {}
    manager: tilt
    operation: Update
    time: "2020-07-07T14:50:17Z"
  name: elastic-system
  resourceVersion: "617"
  selfLink: /api/v1/namespaces/elastic-system
  uid: fa9710ff-7b19-499c-b0f9-faedd1c84969
spec:
  finalizers:
  - kubernetes
`
	entities := mustParseYAML(t, yaml)
	entities[0].Clean()

	result, err := SerializeSpecYAML(entities)
	require.NoError(t, err)

	expected := `apiVersion: v1
kind: Namespace
metadata:
  creationTimestamp: "2020-07-07T14:50:17Z"
  labels:
    app.kubernetes.io/managed-by: tilt
  name: elastic-system
  resourceVersion: "617"
  selfLink: /api/v1/namespaces/elastic-system
  uid: fa9710ff-7b19-499c-b0f9-faedd1c84969
spec:
  finalizers:
  - kubernetes
`
	assert.Equal(t, expected, result)
	if err != nil {
		t.Fatal(err)
	}

}

func entitiesWithKinds(kinds []string) []K8sEntity {
	entities := make([]K8sEntity, len(kinds))
	for i, k := range kinds {
		entities[i] = entityWithKind(k)
	}
	return entities
}

func entityWithKind(kind string) K8sEntity {
	return K8sEntity{
		Obj: fakeObject{
			kind: fakeKind(kind),
		},
	}
}

type fakeObject struct {
	kind fakeKind
}

func (obj fakeObject) GetObjectKind() schema.ObjectKind { return obj.kind }
func (obj fakeObject) DeepCopyObject() runtime.Object   { return obj }

type fakeKind string

func (k fakeKind) SetGroupVersionKind(gvk schema.GroupVersionKind) { panic("unsupported") }
func (k fakeKind) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{Kind: string(k)}
}

func assertKindOrder(t *testing.T, expectedKinds []string, actual []K8sEntity, msg string) {
	require.Len(t, actual, len(expectedKinds), "len(expectedKinds) != len(actualKinds): "+msg)
	actualKinds := make([]string, len(expectedKinds))
	for i, e := range actual {
		actualKinds[i] = e.GVK().Kind
	}
	assert.Equal(t, expectedKinds, actualKinds, msg)
}

func parseYAMLFromStrings(yaml ...string) ([]K8sEntity, error) {
	var res []K8sEntity
	for _, s := range yaml {
		entities, err := ParseYAMLFromString(s)
		if err != nil {
			return nil, err
		}
		res = append(res, entities...)
	}
	return res, nil
}

func mustParseYAML(t *testing.T, yaml string) []K8sEntity {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatalf("ERROR %v parsing k8s YAML:\n%s", err, yaml)
	}
	return entities
}

// TestFilterByMatchesPodTemplateSpecNamespace tests that Services are only matched
// to Deployments in the same namespace. This is a regression test for:
// https://github.com/tilt-dev/tilt/issues/6311
func TestFilterByMatchesPodTemplateSpecNamespace(t *testing.T) {
	// Deployment in namespace "ns-a" with label app=myapp
	deploymentNsA := `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: ns-a
  labels:
    app: myapp
spec:
  selector:
    matchLabels:
      app: myapp
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: myapp
        image: myapp:latest
`

	// Service in namespace "ns-b" with selector app=myapp (different namespace)
	serviceNsB := `
apiVersion: v1
kind: Service
metadata:
  name: myapp
  namespace: ns-b
spec:
  selector:
    app: myapp
  ports:
  - port: 80
`

	// Service in namespace "ns-a" with selector app=myapp (same namespace)
	serviceNsA := `
apiVersion: v1
kind: Service
metadata:
  name: myapp
  namespace: ns-a
spec:
  selector:
    app: myapp
  ports:
  - port: 80
`

	// Service without explicit namespace (should match for backward compatibility)
	serviceNoNs := `
apiVersion: v1
kind: Service
metadata:
  name: myapp
spec:
  selector:
    app: myapp
  ports:
  - port: 80
`

	t.Run("service in different namespace should not match", func(t *testing.T) {
		deployment := mustParseYAML(t, deploymentNsA)[0]
		service := mustParseYAML(t, serviceNsB)[0]

		matches, rest, err := FilterByMatchesPodTemplateSpec(deployment, []K8sEntity{service})
		require.NoError(t, err)
		assert.Empty(t, matches, "service in ns-b should not match deployment in ns-a")
		assert.Len(t, rest, 1, "service should remain in rest")
	})

	t.Run("service in same namespace should match", func(t *testing.T) {
		deployment := mustParseYAML(t, deploymentNsA)[0]
		service := mustParseYAML(t, serviceNsA)[0]

		matches, rest, err := FilterByMatchesPodTemplateSpec(deployment, []K8sEntity{service})
		require.NoError(t, err)
		assert.Len(t, matches, 1, "service in ns-a should match deployment in ns-a")
		assert.Empty(t, rest, "no services should remain")
	})

	t.Run("service without namespace should match any deployment (backward compatibility)", func(t *testing.T) {
		deployment := mustParseYAML(t, deploymentNsA)[0]
		service := mustParseYAML(t, serviceNoNs)[0]

		matches, rest, err := FilterByMatchesPodTemplateSpec(deployment, []K8sEntity{service})
		require.NoError(t, err)
		assert.Len(t, matches, 1, "service without namespace should match deployment in ns-a")
		assert.Empty(t, rest, "no services should remain")
	})

	t.Run("only matching namespace service should be selected", func(t *testing.T) {
		deployment := mustParseYAML(t, deploymentNsA)[0]
		serviceA := mustParseYAML(t, serviceNsA)[0]
		serviceB := mustParseYAML(t, serviceNsB)[0]

		matches, rest, err := FilterByMatchesPodTemplateSpec(deployment, []K8sEntity{serviceA, serviceB})
		require.NoError(t, err)
		assert.Len(t, matches, 1, "only service in ns-a should match")
		assert.Equal(t, "ns-a", string(matches[0].Namespace()))
		assert.Len(t, rest, 1, "service in ns-b should remain")
		assert.Equal(t, "ns-b", string(rest[0].Namespace()))
	})
}
