package k8s

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestInjectLabelPod(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.LonelyPodYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	newEntity, err := InjectLabels(entity, []LabelPair{
		{
			Key:   "tier",
			Value: "test",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := SerializeYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, fmt.Sprintf("tier: test")) {
		t.Errorf("labels did not appear in serialized yaml: %s", result)
	}
}

func TestInjectLabelDeployment(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.SanchoYAML)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}

	entity := entities[0]
	newEntity, err := InjectLabels(entity, []LabelPair{
		{
			Key:   "tier",
			Value: "test",
		},
		{
			Key:   "owner",
			Value: "me",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	result, err := SerializeYAML([]K8sEntity{newEntity})
	if err != nil {
		t.Fatal(err)
	}

	// We expect both the Deployment and the PodTemplate to get the labels.
	assert.Equal(t, 2, strings.Count(result, fmt.Sprintf("tier: test")))
	assert.Equal(t, 2, strings.Count(result, fmt.Sprintf("owner: me")))
}

func TestEntityMatchesLabels(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatal("expected exactly two entities")
	}
	if entities[0].Kind.Kind != "Service" {
		t.Fatal("expected first entity to be a Service")
	}
	if entities[1].Kind.Kind != "Deployment" {
		t.Fatal("expected second entity to be a Deployment")
	}

	svc := entities[0]
	dep := entities[1]

	labels := map[string]string{
		"app":         "blorg",
		"owner":       "nick",
		"environment": "devel",
		"tier":        "backend",
		"foo":         "bar", // an extra label on the pod shouldn't affect the match
	}
	assert.True(t, svc.MatchesLabels(labels))

	assert.False(t, dep.MatchesLabels(labels), "kind Deployment does not support MatchesLabels")

	labels["app"] = "not-blorg"
	assert.False(t, svc.MatchesLabels(labels), "wrong value for an expected key")

	delete(labels, "app")
	assert.False(t, svc.MatchesLabels(labels), "expected key missing")
}
