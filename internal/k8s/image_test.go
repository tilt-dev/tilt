package k8s

import (
	"fmt"
	"strings"
	"testing"
)

func TestExtractSanchoYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	containers, err := extractContainers(&entity)
	if err != nil {
		t.Fatal(err)
	}

	if len(containers) != 1 || containers[0].Image != "gcr.io/some-project-162817/sancho" {
		t.Errorf("Unexpected containers: %v", containers)
	}
}

func TestInjectDigestSanchoYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	name := "gcr.io/some-project-162817/sancho"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	newEntity, replaced, err := InjectImageDigestWithStrings(entity, name, digest)
	if err != nil {
		t.Fatal(err)
	}

	if !replaced {
		t.Errorf("Expected replaced: true. Actual: %v", replaced)
	}

	result, err := SerializeYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, fmt.Sprintf("image: %s@%s", name, digest)) {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}
}
