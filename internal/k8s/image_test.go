package k8s

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
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

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
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

	result, err := SerializeSpecYAML([]K8sEntity{entity})
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

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "imagePullPolicy: Never") {
		t.Errorf("image does not have correct pull policy: %s", result)
	}

	serializedOrigEntity, err := SerializeSpecYAML([]K8sEntity{entity})
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

	result, err := SerializeSpecYAML([]K8sEntity{entity})
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
	newEntity, replaced, err := InjectImageDigest(entity, container.NameSelector(namedTagged), namedTagged, false, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	}

	if !replaced {
		t.Errorf("Expected replaced: true. Actual: %v", replaced)
	}

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
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

// the same as InjectImageDigestInjectRefWithStrings, but with original == inject (the normal case with no default_registry)
func InjectImageDigestWithStrings(entity K8sEntity, original string, newDigest string, policy v1.PullPolicy) (K8sEntity, bool, error) {
	return InjectImageDigestInjectRefWithStrings(entity, original, original, newDigest, policy)
}

// Returns: the new entity, whether anything was replaced, and an error.
func InjectImageDigestInjectRefWithStrings(entity K8sEntity, original string, inject string, newDigest string, policy v1.PullPolicy) (K8sEntity, bool, error) {
	originalRef, err := reference.ParseNamed(original)
	if err != nil {
		return K8sEntity{}, false, err
	}

	injectRef, err := reference.ParseNamed(inject)
	if err != nil {
		return K8sEntity{}, false, err
	}

	d, err := digest.Parse(newDigest)
	if err != nil {
		return K8sEntity{}, false, err
	}

	canonicalRef, err := reference.WithDigest(injectRef, d)
	if err != nil {
		return K8sEntity{}, false, err
	}

	return InjectImageDigest(entity, container.NameSelector(originalRef), canonicalRef, false, policy)
}

func TestInjectSyncletImage(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SyncletYAML)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, len(entities))
	entity := entities[0]
	name := "gcr.io/windmill-public-containers/synclet"
	namedTagged, _ := container.ParseNamedTagged(fmt.Sprintf("%s:tilt-deadbeef", name))
	newEntity, replaced, err := InjectImageDigest(entity, container.NameSelector(namedTagged), namedTagged, false, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	} else if !replaced {
		t.Errorf("Expected replacement in:\n%s", testyaml.SyncletYAML)
	}

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, namedTagged.String()) {
		t.Errorf("could not find image in yaml (%s):\n%s", namedTagged, result)
	}
}

func TestEntityHasImage(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	img := container.MustParseSelector("gcr.io/blorg-dev/blorg-backend:devel-nick")
	wrongImg := container.MustParseSelector("gcr.io/blorg-dev/wrong-app-whoops:devel-nick")

	match, err := entities[0].HasImage(img, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, match, "service yaml should not match (does not contain image)")

	match, err = entities[1].HasImage(img, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, match, "deployment yaml should match image %s", img.String())

	match, err = entities[1].HasImage(wrongImg, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.False(t, match, "deployment yaml should not match image %s", img.String())

	entities, err = ParseYAMLFromString(testyaml.CRDYAML)
	if err != nil {
		t.Fatal(err)
	}

	img = container.MustParseTaggedSelector("docker.io/bitnami/minideb:latest")
	e := entities[0]
	jp, err := NewJSONPath("{.spec.validation.openAPIV3Schema.properties.spec.properties.image}")
	if err != nil {
		t.Fatal(err)
	}
	imageJSONPaths := []JSONPath{jp}
	match, err = e.HasImage(img, imageJSONPaths, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, match, "CRD yaml should match image %s", img.String())

	entities, err = ParseYAMLFromString(testyaml.SanchoImageInEnvYAML)
	if err != nil {
		t.Fatal(err)
	}
	img = container.MustParseSelector("gcr.io/some-project-162817/sancho")
	e = entities[0]
	match, err = e.HasImage(img, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, match, "deployment yaml should match image %s", img.String())
	img2 := container.MustParseSelector("gcr.io/some-project-162817/sancho2")
	e = entities[0]
	match, err = e.HasImage(img2, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, match, "CRD yaml should match image %s", img2.String())

}

func testInjectDigestCRD(t *testing.T, yaml string, expectedDigestPrefix string) {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	name := "gcr.io/foo"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	newEntity, replaced, err := InjectImageDigestWithStrings(entity, name, digest, v1.PullIfNotPresent)
	if err != nil {
		t.Fatal(err)
	}

	if !replaced {
		t.Errorf("Expected replaced: true. Actual: %v", replaced)
	}

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, fmt.Sprintf("%s%s@%s", expectedDigestPrefix, name, digest)) {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}
}

// e.g., using a crd w/ a default_registry
func TestInjectDigestCRDSelectorDoesntMatchInjectRef(t *testing.T) {
	yaml := `
apiversion: foo/v1
kind: Foo
spec:
    image: gcr.io/foo:stable
`

	entities, err := ParseYAMLFromString(yaml)
	require.NoError(t, err)

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	originalName := "gcr.io/foo"
	injectionName := "localhost:3000/gcr_io_foo"
	digest := "sha256:2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	newEntity, replaced, err := InjectImageDigestInjectRefWithStrings(entity, originalName, injectionName, digest, v1.PullIfNotPresent)
	require.NoError(t, err)

	require.Truef(t, replaced, "expected replaced: true. actual: %v", replaced)

	result, err := SerializeSpecYAML([]K8sEntity{newEntity})
	require.NoError(t, err)

	if !strings.Contains(result, fmt.Sprintf("%s%s@%s", "image: ", injectionName, digest)) {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}
}

func TestInjectDigestCRDMapValue(t *testing.T) {
	testInjectDigestCRD(t, `
apiversion: foo/v1
kind: Foo
spec:
    image: gcr.io/foo:stable
`, "image: ")
}

func TestInjectDigestCRDListElement(t *testing.T) {
	testInjectDigestCRD(t, `
apiversion: foo/v1
kind: Foo
spec:
    images:
      - gcr.io/foo:stable
`, "- ")
}

func TestInjectDigestCRDListOfMaps(t *testing.T) {
	testInjectDigestCRD(t, `
apiversion: foo/v1
kind: Foo
spec:
    args:
        image: gcr.io/foo:stable
`, "image: ")
}

func TestMatchInEnvVarsFalse(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoImageInEnvYAML)
	name := "gcr.io/some-project-162817/sancho"
	digest := "2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	namedTagged, err := reference.ParseNamed(fmt.Sprintf("%s:%s", name, digest))
	if err != nil {
		t.Fatal(err)
	}
	newEntity, replaced, err := InjectImageDigest(entity, container.NameSelector(namedTagged), namedTagged, false, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, replaced)
	d := newEntity.Obj.(*appsv1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}
	c := d.Spec.Template.Spec.Containers[0]
	// make sure we didn't inject to the env var
	assert.Equal(t, namedTagged.String(), c.Image)
	assert.Contains(t, c.Env, v1.EnvVar{Name: "bar", Value: name})
}

func TestMatchInEnvVarsTrue(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoImageInEnvYAML)
	name := "gcr.io/some-project-162817/sancho"
	digest := "2baf1f40105d9501fe319a8ec463fdf4325a2a5df445adf3f572f626253678c9"
	namedTagged, err := reference.ParseNamed(fmt.Sprintf("%s:%s", name, digest))
	if err != nil {
		t.Fatal(err)
	}
	newEntity, replaced, err := InjectImageDigest(entity, container.NameSelector(namedTagged), namedTagged, true, v1.PullNever)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, replaced)
	d := newEntity.Obj.(*appsv1.Deployment)
	if !assert.Equal(t, 1, len(d.Spec.Template.Spec.Containers)) {
		return
	}
	c := d.Spec.Template.Spec.Containers[0]
	// make sure we didn't inject to the env var
	assert.Equal(t, namedTagged.String(), c.Image)
	assert.Contains(t, c.Env, v1.EnvVar{Name: "bar", Value: namedTagged.String()})
}
