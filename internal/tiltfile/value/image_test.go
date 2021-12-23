package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestImageListNone(t *testing.T) {
	var v ImageList
	err := v.Unpack(starlark.None)
	require.NoError(t, err)
	require.Nil(t, v)
}

func TestImageListValues(t *testing.T) {
	var v ImageList
	err := v.Unpack(starlark.NewList([]starlark.Value{
		starlark.String("foo"),
		starlark.String("bar"),
		starlark.String("gcr.io/baz"),
	}))
	require.NoError(t, err)
	if assert.Len(t, v, 3) {
		assert.Equal(t, "docker.io/library/foo", v[0].String())
		assert.Equal(t, "docker.io/library/bar", v[1].String())
		assert.Equal(t, "gcr.io/baz", v[2].String())
	}
}
