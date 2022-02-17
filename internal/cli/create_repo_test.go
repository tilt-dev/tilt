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

func TestCreateRepo(t *testing.T) {
	f := newServerFixture(t)

	out := bytes.NewBuffer(nil)
	streams := genericclioptions.IOStreams{Out: out}

	cmd := newCreateRepoCmd(streams)
	c := cmd.register()
	err := c.Flags().Parse([]string{
		"default", "https://github.com/tilt-dev/tilt-extensions",
		"--ref", "FAKE_SHA",
	})
	require.NoError(t, err)

	err = cmd.run(f.ctx, c.Flags().Args())
	require.NoError(t, err)
	assert.Contains(t, out.String(), `extensionrepo.tilt.dev/default created`)

	var obj v1alpha1.ExtensionRepo
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "default"}, &obj)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/tilt-dev/tilt-extensions", obj.Spec.URL)
	assert.Equal(t, "FAKE_SHA", obj.Spec.Ref)
}
