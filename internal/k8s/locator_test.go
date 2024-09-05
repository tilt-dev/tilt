package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestCRDImageObjectInjection(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.CRDImageObjectYAML)
	require.NoError(t, err)

	e := entities[0]
	selector := MustKindSelector("UselessMachine")
	locator := MustJSONPathImageObjectLocator(selector, "{.spec.imageObject}", "repo", "tag", false)
	images, err := locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 1, len(images))
	assert.Equal(t, "docker.io/library/frontend", images[0].String())

	e, modified, err := locator.Inject(e, container.MustParseSelector("frontend"),
		container.MustParseNamed("frontend:tilt-123"), v1.PullNever)
	require.NoError(t, err)
	assert.True(t, modified)

	images, err = locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 1, len(images))
	assert.Equal(t, "docker.io/library/frontend:tilt-123", images[0].String())
}

func TestCRDImageObjectInjectionOptional(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.CRDImageObjectYAML)
	require.NoError(t, err)

	e := entities[0]
	selector := MustKindSelector("UselessMachine")
	locator := MustJSONPathImageObjectLocator(selector, "{.spec.notImageObject}", "repo", "tag", true)
	images, err := locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 0, len(images))
}

func TestCRDImagePathInjectionOptional(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.CRDContainerSpecYAML)
	require.NoError(t, err)

	e := entities[0]
	selector := MustKindSelector("UselessMachine")
	locator := MustJSONPathImageLocator(selector, "{.spec.containers[*].notImage}", true)
	images, err := locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 0, len(images))
}

func TestCRDPullPolicyInjection(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.CRDContainerSpecYAML)
	require.NoError(t, err)

	e := entities[0]
	selector := MustKindSelector("UselessMachine")
	locator := MustJSONPathImageLocator(selector, "{.spec.containers[*].image}", false)
	images, err := locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 1, len(images))
	assert.Equal(t, "docker.io/library/frontend", images[0].String())

	e, modified, err := locator.Inject(e, container.MustParseSelector("frontend"),
		container.MustParseNamed("frontend:tilt-123"), v1.PullNever)
	require.NoError(t, err)
	require.True(t, modified)

	spec := e.Obj.(*unstructured.Unstructured).Object["spec"].(map[string]interface{})
	c := spec["containers"].([]interface{})[0].(map[string]interface{})
	require.Equal(t, "frontend:tilt-123", c["image"])
	require.Equal(t, "Never", c["imagePullPolicy"].(string))
}
