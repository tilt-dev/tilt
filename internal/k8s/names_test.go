package k8s

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
			{"foo", "Deployment", "default", "", "foo:deployment:default::0"},
			{"foo", "Deployment", "default", "", "foo:deployment:default::1"},
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
				gvk := obj.GroupVersionKind()
				entities = append(entities, K8sEntity{Obj: &obj, Kind: &gvk})

				expectedNames = append(expectedNames, w.expectedResourceName)
			}

			actualNames := UniqueNames(entities, 1)
			assert.Equal(t, expectedNames, actualNames)
		})
	}
}
