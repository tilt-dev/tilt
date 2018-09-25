package k8s

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"

	"k8s.io/api/core/v1"
)

func TestInjectDigestSanchoYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SanchoYAML)
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

func TestInjectDigestDoesNotMutateOriginal(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	name := "gcr.io/some-project-162817/sancho"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	_, replaced, err := InjectImageDigestWithStrings(entity, name, digest, v1.PullIfNotPresent)
	if err != nil {
		t.Fatal(err)
	}

	if !replaced {
		t.Errorf("Expected replaced: true. Actual: %v", replaced)
	}

	result, err := SerializeYAML([]K8sEntity{entity})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, fmt.Sprintf("image: %s@%s", name, digest)) {
		t.Errorf("oops! accidentally mutated original entity: %s", result)
	}
}

func TestInjectImagePullPolicy(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
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

	serializedOrigEntity, err := SerializeYAML([]K8sEntity{entity})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(serializedOrigEntity, "imagePullPolicy: Never") {
		t.Errorf("oops! accidentally mutated original entity: %+v", entity)
	}
}

func TestInjectImagePullPolicyDoesNotMutateOriginal(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	entity := entities[1]
	_, err = InjectImagePullPolicy(entity, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	}

	result, err := SerializeYAML([]K8sEntity{entity})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(result, "imagePullPolicy: Never") {
		t.Errorf("oops! accidentally mutated original entity: %+v", entity)
	}
}

func TestErrorInjectDigestBlorgBackendYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 2 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[1]
	name := "gcr.io/blorg-dev/blorg-backend"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	_, _, err = InjectImageDigestWithStrings(entity, name, digest, v1.PullNever)
	if err == nil || !strings.Contains(err.Error(), "INTERNAL TILT ERROR") {
		t.Errorf("Expected internal tilt error, actual: %v", err)
	}
}

func TestInjectDigestBlorgBackendYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 2 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[1]
	name := "gcr.io/blorg-dev/blorg-backend"
	namedTagged, _ := reference.ParseNamed(fmt.Sprintf("%s:wm-tilt", name))
	newEntity, replaced, err := InjectImageDigest(entity, namedTagged, v1.PullNever)
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

	if !strings.Contains(result, fmt.Sprintf("image: %s", namedTagged)) {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}

	if !strings.Contains(result, "imagePullPolicy: Never") {
		t.Errorf("image does not have correct pull policy: %s", result)
	}
}

// Returns: the new entity, whether anything was replaced, and an error.
func InjectImageDigestWithStrings(entity K8sEntity, original string, newDigest string, policy v1.PullPolicy) (K8sEntity, bool, error) {
	originalRef, err := reference.ParseNamed(original)
	if err != nil {
		return K8sEntity{}, false, err
	}

	d, err := digest.Parse(newDigest)
	if err != nil {
		return K8sEntity{}, false, err
	}

	canonicalRef, err := reference.WithDigest(originalRef, d)
	if err != nil {
		return K8sEntity{}, false, err
	}

	return InjectImageDigest(entity, canonicalRef, policy)
}

func TestInjectSyncletImage(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SyncletYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(entities))
	entity := entities[0]
	name := "gcr.io/windmill-public-containers/synclet"
	namedTagged, _ := ParseNamedTagged(fmt.Sprintf("%s:tilt-deadbeef", name))
	newEntity, replaced, err := InjectImageDigest(entity, namedTagged, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	} else if !replaced {
		t.Errorf("Expected replacement in:\n%s", testyaml.SyncletYAML)
	}

	result, err := SerializeYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, namedTagged.String()) {
		t.Errorf("could not find image in yaml (%s):\n%s", namedTagged, result)
	}
}
