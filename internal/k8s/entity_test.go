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

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/kustomize"
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

func TestImmutableFilter(t *testing.T) {
	yaml := fmt.Sprintf("%s\n---\n%s\n---\n%s", testyaml.JobYAML, testyaml.SanchoYAML, testyaml.PodYAML)
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	immEntities := ImmutableEntities(entities)
	if len(immEntities) != 2 {
		t.Fatalf("Expected 2 entities, actual: %d", len(immEntities))
	}

	if immEntities[0].GVK().Kind != "Job" {
		t.Errorf("Expected Job entity, actual: %+v", immEntities)
	}
	if immEntities[1].GVK().Kind != "Pod" {
		t.Errorf("Expected Pod entity, actual: %+v", immEntities)
	}
}

func TestMutableFilter(t *testing.T) {
	yaml := fmt.Sprintf("%s\n---\n%s", testyaml.JobYAML, testyaml.SanchoYAML)
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	results := MutableEntities(entities)
	if len(results) != 1 {
		t.Fatalf("Expected 1 entity, actual: %d", len(results))
	}

	if results[0].GVK().Kind != "Deployment" {
		t.Errorf("Expected Deployment entity, actual: %+v", results)
	}
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
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
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
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
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
		t.Fatalf("expected 2 entites, got %d: %v", len(entities), entities)
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
		t.Run(string(test.name), func(t *testing.T) {
			input := entitiesWithKinds(test.inputKindOrder)
			sorted := SortedEntities(input)
			assertKindOrder(t, test.expectedKindOrder, sorted, "sorted entities")
		})
	}
}

func TestMutableAndImmutableEntities(t *testing.T) {
	for _, test := range []struct {
		name                       string
		inputKindOrder             []string
		expectedMutableKindOrder   []string
		expectedImmutableKindOrder []string
	}{
		{"only mutable",
			[]string{"Deployment", "Namespace", "Service"},
			[]string{"Deployment", "Namespace", "Service"},
			[]string{},
		},
		{"only immutable",
			[]string{"Job", "Pod"},
			[]string{},
			[]string{"Job", "Pod"},
		},
		{"mutable and immutable interspersed",
			[]string{"Deployment", "Job", "Namespace", "Pod", "Service"},
			[]string{"Deployment", "Namespace", "Service"},
			[]string{"Job", "Pod"},
		},
		{"no explicitly sorted kinds are immutable",
			// If any kinds in the explicit sort list are also immutable, things will get weird
			kustomize.OrderFirst,
			kustomize.OrderFirst,
			[]string{},
		},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			input := entitiesWithKinds(test.inputKindOrder)
			mutable, immutable := MutableAndImmutableEntities(input)
			assertKindOrder(t, test.expectedMutableKindOrder, mutable, "mutable entities")
			assertKindOrder(t, test.expectedImmutableKindOrder, immutable, "immutable entities")
		})
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
