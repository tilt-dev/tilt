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
