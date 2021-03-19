package visitor_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/tilt-dev/tilt/internal/cli/visitor"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDecode(t *testing.T) {
	cmds, err := decode(t, `
apiVersion: tilt.dev/v1alpha1
kind: Cmd
metadata:
  name: hello-world
spec:
  args: ["echo", "hello world"]
---
apiVersion: tilt.dev/v1alpha1
kind: Cmd
metadata:
  name: goodbye-world
spec:
  args: ["echo", "goodbye world"]
`)
	require.NoError(t, err)
	require.Equal(t, 2, len(cmds))
	assert.Equal(t, "hello-world", cmds[0].(*v1alpha1.Cmd).Name)
	assert.Equal(t, "goodbye-world", cmds[1].(*v1alpha1.Cmd).Name)
}

func TestDecodeMisspelledField(t *testing.T) {
	_, err := decode(t, `
apiVersion: tilt.dev/v1alpha1
kind: Cmd
metadata:
  name: hello-world
spec:
  misspell: ["echo", "hello world"]
`)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), `unknown field "misspell"`)
		assert.Contains(t, err.Error(), `hello world`)
	}
}

func decode(t *testing.T, s string) ([]runtime.Object, error) {
	visitors, err := visitor.FromStrings([]string{"-"}, strings.NewReader(s))
	require.NoError(t, err)
	return visitor.DecodeAll(v1alpha1.NewScheme(), visitors)
}
