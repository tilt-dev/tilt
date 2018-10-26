package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

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
	yaml := fmt.Sprintf("%s\n---\n%s", testyaml.JobYAML, testyaml.SanchoYAML)
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	jobs := ImmutableEntities(entities)
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 entity, actual: %d", len(jobs))
	}

	if jobs[0].Kind.Kind != "Job" {
		t.Errorf("Expected Job entity, actual: %+v", jobs)
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

	if results[0].Kind.Kind != "Deployment" {
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
		if e.Kind.Kind == "Deployment" || e.Kind.Kind == "Job" {
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
	popped, rest, err = Filter(entities, returnErr)
	if assert.Error(t, err, "expected Filter to propagate err from test func") {
		assert.Equal(t, err.Error(), "omgwtfbbq")
	}
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
