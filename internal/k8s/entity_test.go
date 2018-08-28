package k8s

import (
	"fmt"
	"testing"
)

func TestImmutableFilter(t *testing.T) {
	yaml := fmt.Sprintf("%s\n---\n%s", JobYAML, SanchoYAML)
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

func TestLoadBalancers(t *testing.T) {
	entities, err := ParseYAMLFromString(BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	lbs := ToLoadBalancers(entities)
	if len(lbs) != 1 {
		t.Fatalf("Expected 1 loadbalancer, actual %d: %v", len(lbs), lbs)
	}

	if lbs[0].Name != "devel-nick-lb-blorg-be" ||
		lbs[0].Ports[0] != 8080 {
		t.Fatalf("Unexpected loadbalancer: %+v", lbs[0])
	}
}
