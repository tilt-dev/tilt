package value

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestAsStringOrStringList_String(t *testing.T) {
	actual, err := AsStringOrStringList(starlark.String("foo"))

	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, actual)
}

func TestAsStringOrStringList_ListOfStrings(t *testing.T) {
	actual, err := AsStringOrStringList(starlark.NewList([]starlark.Value{
		starlark.String("foo"),
		starlark.String("bar"),
		starlark.String("baz"),
	}))

	require.NoError(t, err)
	require.Equal(t, []string{"foo", "bar", "baz"}, actual)
}

func TestAsStringOrStringList_NonStringOrList(t *testing.T) {
	_, err := AsStringOrStringList(starlark.Bool(true))
	require.Error(t, err)
	require.Contains(t, err.Error(), "value should be a string or List of strings, but is of type bool")
}

func TestAsStringOrStringList_ListWithNonStringElement(t *testing.T) {
	_, err := AsStringOrStringList(starlark.NewList([]starlark.Value{starlark.String("foo"), starlark.Bool(true)}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "list should contain only strings, but element \"True\" was of type bool")
}

// https://github.com/tilt-dev/tilt/issues/3570
func TestAsStringOrStringList_Map(t *testing.T) {
	m := starlark.NewDict(2)
	err := m.SetKey(starlark.String("foo"), starlark.String("1"))
	require.NoError(t, err)
	err = m.SetKey(starlark.String("bar"), starlark.String("2"))
	require.NoError(t, err)

	_, err = AsStringOrStringList(m)
	require.Error(t, err)
	require.Contains(t, err.Error(), "value should be a string or List of strings, but is of type dict")
}
