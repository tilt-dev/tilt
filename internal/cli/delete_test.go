package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDelete(t *testing.T) {
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
	deleteCmd := newDeleteCmd()
	deleteCmd.register()
	deleteCmd.streams.Out = out

	err = deleteCmd.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `cmd.tilt.dev "my-sleep" deleted`)

	var cmd v1alpha1.Cmd
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "my-sleep"}, &cmd)
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}
