package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreateRepo(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	out := bytes.NewBuffer(nil)

	cmd := newCreateRepoCmd()
	cmd.helper.streams.Out = out
	c := cmd.register()
	err := c.Flags().Parse([]string{
		"--ref", "FAKE_SHA",
	})
	require.NoError(t, err)

	err = cmd.run(f.ctx, []string{"default", "https://github.com/tilt-dev/tilt-extensions"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `extensionrepo.tilt.dev/default created`)

	var obj v1alpha1.ExtensionRepo
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "default"}, &obj)
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/tilt-dev/tilt-extensions", obj.Spec.URL)
	assert.Equal(t, "FAKE_SHA", obj.Spec.Ref)
}
