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
