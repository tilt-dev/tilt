package k8s

import (
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestTracerYAML(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.TracerYAML)
	if err != nil {
		t.Fatal(err)
	}

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
	entities, err := ParseYAMLFromString(testyaml.SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

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
