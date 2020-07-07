package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
)

func TestCRDImageObjectInjection(t *testing.T) {
	entities, err := ParseYAMLFromString(testyaml.CRDImageObjectYAML)
	require.NoError(t, err)

	e := entities[0]
	selector := MustKindSelector("UselessMachine")
	locator := MustJSONPathImageObjectLocator(selector, "{.spec.imageObject}", "repo", "tag")
	images, err := locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 1, len(images))
	assert.Equal(t, "docker.io/library/frontend", images[0].String())

	e, modified, err := locator.Inject(e, container.MustParseSelector("frontend"),
		container.MustParseNamed("frontend:tilt-123"))
	require.NoError(t, err)
	assert.True(t, modified)

	images, err = locator.Extract(e)
	require.NoError(t, err)
	require.Equal(t, 1, len(images))
	assert.Equal(t, "docker.io/library/frontend:tilt-123", images[0].String())
}
