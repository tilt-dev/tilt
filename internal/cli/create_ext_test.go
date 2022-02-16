package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreateExt(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	out := bytes.NewBuffer(nil)

	streams := genericclioptions.IOStreams{Out: out}
	cmd := newCreateExtCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{
		"cancel",
		"--repo", "my-repo",
		"--path", "my-path",
		"--",
		"foo",
		"--namespace=bar",
	})
	require.NoError(t, err)

	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)
	assert.Contains(t, out.String(), `extension.tilt.dev/cancel created`)

	var obj v1alpha1.Extension
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "cancel"}, &obj)
	require.NoError(t, err)

	assert.Equal(t, "my-repo", obj.Spec.RepoName)
	assert.Equal(t, "my-path", obj.Spec.RepoPath)
	assert.Equal(t, []string{"foo", "--namespace=bar"}, obj.Spec.Args)
}
