package k8s

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/internal/k8s/jsonpath"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestJSONPathOneMatch(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.SanchoYAML)
	deployment := entities[0]
	path, err := NewJSONPath("{.spec.template.spec.containers[].image}")
	assert.NoError(t, err)
	result, err := path.FindStrings(deployment.Obj)
	assert.NoError(t, err)
	assert.Equal(t, []string{"gcr.io/some-project-162817/sancho"}, result)
}

func TestJSONPathReplace(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.SanchoYAML)
	deployment := entities[0]
	path, err := NewJSONPath("{.spec.template.spec.containers[].image}")
	assert.NoError(t, err)

	err = path.VisitStrings(deployment.Obj, func(val jsonpath.Value, s string) error {
		val.Set(reflect.ValueOf("injected-image"))
		return nil
	})
	assert.NoError(t, err)

	result, err := path.FindStrings(deployment.Obj)
	assert.NoError(t, err)
	assert.Equal(t, []string{"injected-image"}, result)
}

func TestJSONPathMultipleMatches(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.SanchoSidecarYAML)
	deployment := entities[0]
	path, err := NewJSONPath("{.spec.template.spec.containers[*].image}")
	assert.NoError(t, err)
	result, err := path.FindStrings(deployment.Obj)
	assert.NoError(t, err)
	assert.Equal(t, []string{
		"gcr.io/some-project-162817/sancho",
		"gcr.io/some-project-162817/sancho-sidecar",
	}, result)
}

func TestJSONPathCRD(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.CRDYAML)
	crd := entities[1]
	path, err := NewJSONPath("{.spec.image}")
	assert.NoError(t, err)

	content := crd.Obj.(runtime.Unstructured).UnstructuredContent()
	result, err := path.FindStrings(content)
	assert.NoError(t, err)
	assert.Equal(t, []string{"docker.io/bitnami/minideb:latest"}, result)
}

func TestJSONPathCRDReplace(t *testing.T) {
	entities := MustParseYAMLFromString(t, testyaml.CRDYAML)
	crd := entities[1]
	path, err := NewJSONPath("{.spec.image}")
	assert.NoError(t, err)

	content := crd.Obj.(runtime.Unstructured).UnstructuredContent()
	err = path.VisitStrings(content, func(val jsonpath.Value, s string) error {
		val.Set(reflect.ValueOf("injected-image"))
		return nil
	})
	assert.NoError(t, err)

	result, err := path.FindStrings(content)
	assert.NoError(t, err)
	assert.Equal(t, []string{"injected-image"}, result)
}
