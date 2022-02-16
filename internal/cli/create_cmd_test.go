package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreateCmd(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	out := bytes.NewBuffer(nil)

	streams := genericclioptions.IOStreams{Out: out}
	cmd := newCreateCmdCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{
		"--env", "COLOR=1",
		"-e", "USER=nick",
		"my-cmd", "echo", "hello", "world",
	})
	require.NoError(t, err)

	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)
	assert.Contains(t, out.String(), `cmd.tilt.dev/my-cmd created`)

	var myCmd v1alpha1.Cmd
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "my-cmd"}, &myCmd)
	require.NoError(t, err)

	cwd, _ := os.Getwd()
	assert.Equal(t, cwd, myCmd.Spec.Dir)
	assert.Equal(t, []string{"echo", "hello", "world"}, myCmd.Spec.Args)
	assert.Equal(t, []string{"COLOR=1", "USER=nick"}, myCmd.Spec.Env)
}
