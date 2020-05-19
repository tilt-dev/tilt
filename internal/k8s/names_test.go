package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

type workload struct {
	name                 string
	kind                 string
	namespace            string
	group                string
	expectedResourceName string
}

func TestUniqueResourceNames(t *testing.T) {
	testCases := []struct {
		testName  string
		workloads []workload
	}{
		{"one workload, just name", []workload{
			{"foo", "Deployment", "default", "", "foo"},
		}},
		{"one workload, same name", []workload{
			{"foo", "Deployment", "default", "", "foo:deployment:default:core:0"},
			{"foo", "Deployment", "default", "", "foo:deployment:default:core:1"},
		}},
		{"one workload, by name", []workload{
			{"foo", "Deployment", "default", "", "foo"},
			{"bar", "Deployment", "default", "", "bar"},
		}},
		{"two workloads, by kind", []workload{
			{"foo", "Deployment", "default", "", "foo:deployment"},
			{"foo", "CronJob", "default", "", "foo:cronjob"},
		}},
		{"two workloads, by namespace", []workload{
			{"foo", "Deployment", "default", "", "foo:deployment:default"},
			{"foo", "Deployment", "fission", "", "foo:deployment:fission"},
		}},
		{"two workloads, by group", []workload{
			{"foo", "Deployment", "default", "a", "foo:deployment:default:a"},
			{"foo", "Deployment", "default", "b", "foo:deployment:default:b"},
		}},
		{"three workloads, one by kind, two by namespace", []workload{
			{"foo", "Deployment", "default", "a", "foo:deployment:default"},
			{"foo", "Deployment", "fission", "b", "foo:deployment:fission"},
			{"foo", "CronJob", "default", "b", "foo:cronjob"},
		}},
	}

	for _, test := range testCases {
		t.Run(test.testName, func(t *testing.T) {
			var entities []K8sEntity
			var expectedNames []string
			for _, w := range test.workloads {
				obj := unstructured.Unstructured{}
				obj.SetName(w.name)
				obj.SetNamespace(w.namespace)
				obj.SetKind(w.kind)
				obj.SetAPIVersion(fmt.Sprintf("%s/1.0", w.group))
				entities = append(entities, NewK8sEntity(&obj))

				expectedNames = append(expectedNames, w.expectedResourceName)
			}

			actualNames := UniqueNames(entities, 1)
			assert.Equal(t, expectedNames, actualNames)
		})
	}
}

func TestFragmentsToEntities(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendYAML)
	if err != nil {
		t.Fatal(err)
	}

	actual := FragmentsToEntities(entities)

	expected := map[string][]K8sEntity{
		"devel-nick-lb-blorg-be":                      {entities[0]},
		"devel-nick-lb-blorg-be:service":              {entities[0]},
		"devel-nick-lb-blorg-be:service:default":      {entities[0]},
		"devel-nick-lb-blorg-be:service:default:core": {entities[0]},

		"devel-nick-blorg-be":                               {entities[1]},
		"devel-nick-blorg-be:deployment":                    {entities[1]},
		"devel-nick-blorg-be:deployment:default":            {entities[1]},
		"devel-nick-blorg-be:deployment:default:extensions": {entities[1]},
	}

	assert.Equal(t, expected, actual)
}

func TestFragmentsToEntitiesAmbiguous(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.BlorgBackendAmbiguousYAML)
	if err != nil {
		t.Fatal(err)
	}

	actual := FragmentsToEntities(entities)

	expected := map[string][]K8sEntity{
		"blorg":                      {entities[0], entities[1]},
		"blorg:service":              {entities[0]},
		"blorg:service:default":      {entities[0]},
		"blorg:service:default:core": {entities[0]},

		"blorg:deployment":                    {entities[1]},
		"blorg:deployment:default":            {entities[1]},
		"blorg:deployment:default:extensions": {entities[1]},
	}

	assert.Equal(t, expected, actual)
}
