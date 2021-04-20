package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreate(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	f.WriteFile("sleep.yaml", `
apiVersion: tilt.dev/v1alpha1
kind: Cmd
metadata:
  name: my-sleep
spec:
  args: ["sleep", "1"]
`)
	out := bytes.NewBuffer(nil)

	cmd := newCreateCmd()
	cmd.streams.Out = out
	c := cmd.register()
	err := c.Flags().Parse([]string{"-f", f.JoinPath("sleep.yaml")})
	require.NoError(t, err)

	err = cmd.run(f.ctx, nil)
	require.NoError(t, err)
	assert.Contains(t, out.String(), `cmd.tilt.dev/my-sleep created`)

	var sleep v1alpha1.Cmd
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "my-sleep"}, &sleep)
	require.NoError(t, err)
	assert.Equal(t, []string{"sleep", "1"}, sleep.Spec.Args)
}
