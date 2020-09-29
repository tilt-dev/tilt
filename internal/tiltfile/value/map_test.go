package value

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestStringStringMap(t *testing.T) {
	sv := starlark.NewDict(2)
	err := sv.SetKey(starlark.String("a"), starlark.String("b"))
	require.NoError(t, err)
	err = sv.SetKey(starlark.String("c"), starlark.String("d"))
	require.NoError(t, err)

	v := StringStringMap{}

	err = v.Unpack(sv)
	require.NoError(t, err)

	expected := StringStringMap{"a": "b", "c": "d"}
	require.Equal(t, expected, v)
}

func TestStringStringMapNotDict(t *testing.T) {
	sv := starlark.NewList([]starlark.Value{starlark.String("a"), starlark.String("b")})

	v := StringStringMap{}

	err := v.Unpack(sv)
	require.Error(t, err)
	require.Contains(t, err.Error(), "expected dict, got *starlark.List")
}

func TestStringStringMapKeyNotString(t *testing.T) {
	sv := starlark.NewDict(1)
	err := sv.SetKey(starlark.MakeInt(1), starlark.String("a"))
	require.NoError(t, err)

	v := StringStringMap{}

	err = v.Unpack(sv)
	require.Error(t, err)
	require.Contains(t, err.Error(), "key is not a string: starlark.Int (1)")
}

func TestStringStringMapValueNotString(t *testing.T) {
	sv := starlark.NewDict(1)
	err := sv.SetKey(starlark.String("a"), starlark.MakeInt(1))
	require.NoError(t, err)

	v := StringStringMap{}

	err = v.Unpack(sv)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value is not a string: starlark.Int (1)")
}

func TestStringStringMapUnpackClearsExistingData(t *testing.T) {
	sv := starlark.NewDict(2)
	err := sv.SetKey(starlark.String("a"), starlark.String("b"))
	require.NoError(t, err)
	err = sv.SetKey(starlark.String("c"), starlark.String("d"))
	require.NoError(t, err)

	v := StringStringMap{}

	err = v.Unpack(sv)
	require.NoError(t, err)

	sv = starlark.NewDict(0)
	err = v.Unpack(sv)
	require.NoError(t, err)
	require.Equal(t, 0, len(v))
}
