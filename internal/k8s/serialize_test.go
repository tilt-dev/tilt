package k8s

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func MustParseYAMLFromString(t *testing.T, s string) []K8sEntity {
	entities, err := ParseYAMLFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return entities
}

func TestTracerYAML(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.TracerYAML)
	if len(entities) != 3 ||
		entities[0].Kind.Kind != "Deployment" ||
		entities[1].Kind.Kind != "Service" ||
		entities[2].Kind.Kind != "Service" {
		t.Errorf("Unexpected entities: %+v", entities)
	}

	result, err := SerializeYAML(entities)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "image: openzipkin/zipkin") {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}
	if !strings.Contains(result, "name: tracer-prod") {
		t.Errorf("service name did not appear in serialized yaml: %s", result)
	}
}

func TestSanchoYAML(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.SanchoYAML)
	if len(entities) != 1 || entities[0].Kind.Kind != "Deployment" {
		t.Errorf("Unexpected entities: %+v", entities)
	}

	result, err := SerializeYAML(entities)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "image: gcr.io/some-project-162817/sancho") {
		t.Errorf("image name did not appear in serialized yaml: %s", result)
	}
}

func TestHelmGeneratedRedisYAML(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.HelmGeneratedRedisYAML)
	assert.Equal(t, 7, len(entities))

	kinds := []string{}
	for _, entity := range entities {
		kinds = append(kinds, entity.Kind.Kind)
	}
	assert.Equal(t, []string{
		"Secret",
		"ConfigMap",
		"ConfigMap",
		"Service",
		"Service",
		"Deployment",
		"StatefulSet",
	}, kinds)
}

func TestCRDYAML(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.CRDYAML)
	assert.Equal(t, 2, len(entities))

	kinds := []string{}
	names := []string{}
	for _, entity := range entities {
		kinds = append(kinds, entity.Kind.Kind)
		names = append(names, entity.Name())
	}
	assert.Equal(t, []string{
		"CustomResourceDefinition",
		"Project",
	}, kinds)
	assert.Equal(t, []string{
		"projects.example.martin-helmich.de",
		"example-project",
	}, names)

	result, err := SerializeYAML(entities)
	if err != nil {
		t.Fatal(err)
	}
	expected := strings.TrimSpace(testyaml.CRDYAML)
	result = strings.TrimSpace(result)
	assert.Equal(t, expected, result)
}
