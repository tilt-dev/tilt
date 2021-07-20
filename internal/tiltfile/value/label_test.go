package value

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.starlark.net/starlark"
)

func TestLabelString(t *testing.T) {
	v := LabelSet{}
	err := v.Unpack(starlark.String("label1"))
	require.NoError(t, err)

	expected := LabelSet{Values: map[string]string{"label1": "label1"}}
	require.Equal(t, expected, v)
}

func TestLabelStringList(t *testing.T) {
	sv := starlark.NewList([]starlark.Value{starlark.String("label1"), starlark.String("label2")})
	v := LabelSet{}
	err := v.Unpack(sv)
	require.NoError(t, err)

	expected := LabelSet{Values: map[string]string{"label1": "label1"}}
	require.Equal(t, expected, v)
}

func TestLabelInvalidName(t *testing.T) {
	v := LabelSet{}
	err := v.Unpack(starlark.String("?0987wrong2345!"))

	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid label")
	require.Contains(t, err.Error(), "alphanumeric characters")
}

func TestLabelInvalidType(t *testing.T) {
	v := LabelSet{}
	err := v.Unpack(starlark.NewDict(1))

	require.Error(t, err)
	require.Contains(t, err.Error(), "value should be a label or List or Tuple of labels")
}

func TestLabelEmptyString(t *testing.T) {
	v := LabelSet{}
	err := v.Unpack(starlark.String(""))

	require.Error(t, err)
	require.Contains(t, err.Error(), "name part must be non-empty")
}

// TODO(lizz): Add test case and logic to error on an empty label list
