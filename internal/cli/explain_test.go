package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestExplain(t *testing.T) {
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
	explain := newExplainCmd()
	explain.register()
	explain.options.IOStreams.Out = out

	err = explain.run(f.ctx, []string{"cmd"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `Cmd represents a process on the host machine.`)
}
