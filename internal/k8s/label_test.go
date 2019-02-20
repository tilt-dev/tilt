package k8s

import (
	"fmt"
	"strings"
	"testing"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
)

func TestInjectLabelPod(t *testing.T) {
	entity := parseOneEntity(t, testyaml.LonelyPodYAML)
	newEntity, err := InjectLabels(entity, []model.LabelPair{
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
	entity := parseOneEntity(t, testyaml.SanchoYAML)
	newEntity, err := InjectLabels(entity, []model.LabelPair{
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
	assert.Equal(t, 2, strings.Count(result, "tier: test"))
	assert.Equal(t, 2, strings.Count(result, "owner: me"))
}

func TestInjectLabelDeploymentBeta1(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoBeta1YAML)
	newEntity, err := InjectLabels(entity, []model.LabelPair{
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

	assert.Equal(t, 2, strings.Count(result, "owner: me"))

	// Assert that matchLabels were injected
	assert.Contains(t, result, "matchLabels")
	assert.Equal(t, 2, strings.Count(testyaml.SanchoBeta1YAML, "app: sancho"))
	assert.Equal(t, 3, strings.Count(result, "app: sancho"))
}

func TestInjectLabelDeploymentBeta2(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoBeta2YAML)
	newEntity, err := InjectLabels(entity, []model.LabelPair{
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

	assert.Equal(t, 2, strings.Count(result, "owner: me"))

	// Assert that matchLabels were injected
	assert.Contains(t, result, "matchLabels")
	assert.Equal(t, 2, strings.Count(testyaml.SanchoBeta1YAML, "app: sancho"))
	assert.Equal(t, 3, strings.Count(result, "app: sancho"))
}

func TestInjectLabelExtDeploymentBeta1(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoExtBeta1YAML)
	newEntity, err := InjectLabels(entity, []model.LabelPair{
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

	assert.Equal(t, 2, strings.Count(result, "owner: me"))

	// Assert that matchLabels were injected
	assert.Contains(t, result, "matchLabels")
	assert.Equal(t, 2, strings.Count(testyaml.SanchoBeta1YAML, "app: sancho"))
	assert.Equal(t, 3, strings.Count(result, "app: sancho"))
}

func TestSelectorMatchesLabels(t *testing.T) {
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
	assert.True(t, svc.SelectorMatchesLabels(labels))

	assert.False(t, dep.SelectorMatchesLabels(labels), "kind Deployment does not support SelectorMatchesLabels")

	labels["app"] = "not-blorg"
	assert.False(t, svc.SelectorMatchesLabels(labels), "wrong value for an expected key")

	delete(labels, "app")
	assert.False(t, svc.SelectorMatchesLabels(labels), "expected key missing")
}

func TestMatchesMetadataLabels(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.DoggosServiceYaml)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatal("expected exactly two entities")
	}
	e := entities[0]

	exactMatch := map[string]string{
		"app":          "doggos",
		"whosAGoodBoy": "imAGoodBoy",
	}
	assertMatchesMetadataLabels(t, e, exactMatch, true, "same set of labels should match")

	subset := map[string]string{
		"app": "doggos",
	}
	assertMatchesMetadataLabels(t, e, subset, true, "subset of labels should match")

	labelsWithExtra := map[string]string{
		"app":           "doggos",
		"whosAGoodBoy":  "imAGoodBoy",
		"tooManyLabels": "yep",
	}
	assertMatchesMetadataLabels(t, e, labelsWithExtra, false, "extra key not in metadata")

	wrongValForKey := map[string]string{
		"app":          "doggos",
		"whosAGoodBoy": "notMeWhoops",
	}
	assertMatchesMetadataLabels(t, e, wrongValForKey, false, "label with wrong val for key")
}

func assertMatchesMetadataLabels(t *testing.T, e K8sEntity, labels map[string]string, expected bool, msg string) {
	match, err := e.MatchesMetadataLabels(labels)
	if err != nil {
		t.Errorf("error checking if entity %s matches labels %v: %v", e.Name(), labels, err)
	}
	assert.Equal(t, expected, match, "expected entity %s matches metadata labels %v --> %t (%s)",
		e.Name(), labels, expected, msg)
}
func parseOneEntity(t *testing.T, yaml string) K8sEntity {
	entities, err := ParseYAMLFromString(yaml)
	if err != nil {
		t.Fatal(err)
	}

	if len(entities) != 1 {
		t.Fatalf("Unexpected entities: %+v", entities)
	}
	return entities[0]
}
