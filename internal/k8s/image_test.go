package k8s

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/api/core/v1"
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
	newEntity, replaced, err := InjectImageDigestWithStrings(entity, name, digest, v1.PullIfNotPresent)
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

func TestInjectImagePullPolicy(t *testing.T) {
	entities, err := ParseYAMLFromString(BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	entity := entities[1]
	newEntity, err := InjectImagePullPolicy(entity, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SerializeYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "imagePullPolicy: Never") {
		t.Errorf("image does not have correct pull policy: %s", result)
	}
}

func TestInjectDigestBlorgBackendYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 2 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[1]
	name := "gcr.io/blorg-dev/blorg-backend"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	newEntity, replaced, err := InjectImageDigestWithStrings(entity, name, digest, v1.PullNever)
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

	if !strings.Contains(result, "imagePullPolicy: Never") {
		t.Errorf("image does not have correct pull policy: %s", result)
	}
}
