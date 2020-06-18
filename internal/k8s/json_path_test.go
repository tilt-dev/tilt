package k8s

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

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

	err = path.VisitStrings(deployment.Obj, func(val reflect.Value) error {
		val.SetString("injected-image")
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
