package k8s

import (
	"testing"

	extbeta1 "k8s.io/api/extensions/v1beta1"

	"k8s.io/api/apps/v1beta1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta2"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/pkg/model"
)

type field struct {
	name string
	m    map[string]string
}

func verifyFields(t *testing.T, expected []model.LabelPair, fields []field) {
	em := make(map[string]string)
	for _, l := range expected {
		em[l.Key] = l.Value
	}

	for _, f := range fields {
		require.Equal(t, em, f.m, f.name)
	}
}

func TestInjectLabelPod(t *testing.T) {
	entity := parseOneEntity(t, testyaml.LonelyPodYAML)
	lps := []model.LabelPair{
		{
			Key:   "tier",
			Value: "test",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	p, ok := newEntity.Obj.(*v1.Pod)
	require.True(t, ok)

	verifyFields(t, lps, []field{{"pod.Labels", p.Labels}})
}

func TestInjectLabelDeployment(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoYAML)
	lps := []model.LabelPair{
		{Key: "tier", Value: "test"},
		{Key: "owner", Value: "me"},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	d, ok := newEntity.Obj.(*appsv1.Deployment)
	require.True(t, ok)

	appLP := model.LabelPair{Key: "app", Value: "sancho"}
	expectedLPs := append(lps, appLP)

	verifyFields(t, expectedLPs, []field{
		{"d.Labels", d.Labels},
		{"d.Spec.Template.Labels", d.Spec.Template.Labels},
	})
	// matchlabels is not updated
	verifyFields(t, []model.LabelPair{appLP}, []field{
		{"d.Spec.Selector.MatchLabels", d.Spec.Selector.MatchLabels},
	})
}

func TestInjectLabelDeploymentMakeSelectorMatchOnConflict(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoYAML)
	lps := []model.LabelPair{
		{
			Key:   "app",
			Value: "panza",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	d, ok := newEntity.Obj.(*appsv1.Deployment)
	require.True(t, ok)

	verifyFields(t, lps, []field{
		{"d.Labels", d.Labels},
		{"d.Spec.Template.Labels", d.Spec.Template.Labels},
	})
	// matchlabels only gets its existing 'app' label updated, it doesn't get any new labels added
	verifyFields(t, []model.LabelPair{{Key: "app", Value: "panza"}}, []field{
		{"d.Spec.Selector.MatchLabels", d.Spec.Selector.MatchLabels},
	})
}

func TestInjectLabelDeploymentBeta1(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoBeta1YAML)
	lps := []model.LabelPair{
		{
			Key:   "owner",
			Value: "me",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	d, ok := newEntity.Obj.(*v1beta1.Deployment)
	require.True(t, ok)

	expectedLPs := append(lps, model.LabelPair{Key: "app", Value: "sancho"})

	verifyFields(t, expectedLPs, []field{
		{"d.Labels", d.Labels},
		{"d.Spec.Template.Labels", d.Spec.Template.Labels},
		{"d.Spec.Selector.MatchLabels", d.Spec.Selector.MatchLabels},
	})
}

func TestInjectLabelDeploymentBeta2(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoBeta2YAML)
	lps := []model.LabelPair{
		{
			Key:   "owner",
			Value: "me",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	d, ok := newEntity.Obj.(*v1beta2.Deployment)
	require.True(t, ok)

	expectedLPs := append(lps, model.LabelPair{Key: "app", Value: "sancho"})

	verifyFields(t, expectedLPs, []field{
		{"d.Labels", d.Labels},
		{"d.Spec.Template.Labels", d.Spec.Template.Labels},
		{"d.Spec.Selector.MatchLabels", d.Spec.Selector.MatchLabels},
	})
}

func TestInjectLabelExtDeploymentBeta1(t *testing.T) {
	entity := parseOneEntity(t, testyaml.SanchoExtBeta1YAML)
	lps := []model.LabelPair{
		{
			Key:   "owner",
			Value: "me",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	d, ok := newEntity.Obj.(*extbeta1.Deployment)
	require.True(t, ok)

	expectedLPs := append(lps, model.LabelPair{Key: "app", Value: "sancho"})

	verifyFields(t, expectedLPs, []field{
		{"d.Labels", d.Labels},
		{"d.Spec.Template.Labels", d.Spec.Template.Labels},
		{"d.Spec.Selector.MatchLabels", d.Spec.Selector.MatchLabels},
	})
}

func TestInjectStatefulSet(t *testing.T) {
	entity := parseOneEntity(t, testyaml.RedisStatefulSetYAML)
	lps := []model.LabelPair{
		{
			Key:   "tilt-runid",
			Value: "deadbeef",
		},
	}
	newEntity, err := InjectLabels(entity, lps)
	if err != nil {
		t.Fatal(err)
	}

	expectedLPs := append(lps, []model.LabelPair{
		{Key: "app", Value: "redis"},
		{Key: "chart", Value: "redis-5.1.3"},
		{Key: "release", Value: "test"},
	}...)

	ss := newEntity.Obj.(*v1beta2.StatefulSet)
	verifyFields(t, append(expectedLPs, model.LabelPair{Key: "heritage", Value: "Tiller"}), []field{
		{"ss.Labels", ss.Labels},
	})
	verifyFields(t, append(expectedLPs, model.LabelPair{Key: "role", Value: "master"}), []field{
		{"ss.Spec.Template.Labels", ss.Spec.Template.Labels},
	})
	verifyFields(t,
		[]model.LabelPair{
			{Key: "app", Value: "redis"},
			{Key: "release", Value: "test"},
			{Key: "role", Value: "master"},
		}, []field{
			{"ss.Spec.Selector.MatchLabels", ss.Spec.Selector.MatchLabels},
		})

	verifyFields(t,
		[]model.LabelPair{
			{Key: "app", Value: "redis"},
			{Key: "component", Value: "master"},
			{Key: "heritage", Value: "Tiller"},
			{Key: "release", Value: "test"},
		}, []field{
			{"ss.Spec.VolumeClaimTemplates[0].ObjectMeta.Labels", ss.Spec.VolumeClaimTemplates[0].ObjectMeta.Labels},
		})
}

func TestInjectService(t *testing.T) {
	entity := parseOneEntity(t, testyaml.DoggosServiceYaml)
	lps := []model.LabelPair{
		{Key: "foo", Value: "bar"},
		{Key: "app", Value: "cattos"},
	}
	newEntity, err := InjectLabels(entity, lps)
	require.NoError(t, err)

	svc, ok := newEntity.Obj.(*v1.Service)
	require.True(t, ok)

	expectedLPs := append(lps, model.LabelPair{Key: "whosAGoodBoy", Value: "imAGoodBoy"})
	verifyFields(t, expectedLPs, []field{
		{"svc.Labels", svc.Labels},
	})

	// selector only gets existing labels updated
	verifyFields(t, []model.LabelPair{{Key: "app", Value: "cattos"}}, []field{
		{"svc.Spec.Selector", svc.Spec.Selector},
	})
}

func TestSelectorMatchesLabels(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatal("expected exactly two entities")
	}
	if entities[0].GVK().Kind != "Service" {
		t.Fatal("expected first entity to be a Service")
	}
	if entities[1].GVK().Kind != "Deployment" {
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
