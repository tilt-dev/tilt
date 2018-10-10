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
