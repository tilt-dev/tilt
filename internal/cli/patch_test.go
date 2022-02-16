package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestPatch(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	err := f.client.Create(f.ctx, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"sleep", "1"},
		},
	})
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	streams := genericclioptions.IOStreams{Out: out}

	cmd := newPatchCmd(streams)
	c := cmd.register()
	err = c.Flags().Parse([]string{"-p", `{"spec": {"dir": "/tmp"}}`})
	require.NoError(t, err)

	err = cmd.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `cmd.tilt.dev/my-sleep patched`)

	var sleep v1alpha1.Cmd
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "my-sleep"}, &sleep)
	require.NoError(t, err)
	assert.Equal(t, "/tmp", sleep.Spec.Dir)
}
