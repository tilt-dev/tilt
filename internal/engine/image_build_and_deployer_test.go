package engine

import (
	"testing"

	"github.com/docker/distribution/reference"
	"github.com/magiconair/properties/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	v1beta13 "k8s.io/api/extensions/v1beta1"
)

var testTag = "blah"

func TestUpdateK8sEntities(t *testing.T) {
	entities, err := k8s.ParseYAMLFromString(SanchoService.K8sYaml)
	if err != nil {
		t.Fatal(err)
	}

	named, err := reference.ParseNamed(SanchoService.DockerfileTag)
	if err != nil {
		t.Fatal("reference.ParseNamed", err)
	}
	namedTagged, err := reference.WithTag(named, testTag)
	if err != nil {
		t.Fatal("reference.WithTag", err)
	}

	newEntities, err := updateK8sEntities(entities, SanchoService, namedTagged, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(newEntities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(newEntities))
	}

	depl, ok := newEntities[0].Obj.(*v1beta13.Deployment)
	if !ok {
		t.Errorf("expected entity to be of type `*v1beta13.Deployment`, got %T", newEntities[0].Obj)
	}

	assert.Equal(t, depl.Spec.Template.Labels[servNameLabel], SanchoService.Name.String(), "pod template label: %s", servNameLabel)

	containers := depl.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}
	assert.Equal(t, containers[0].Image, namedTagged.String(), "container image")

}
