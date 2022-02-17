package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestWait(t *testing.T) {
	f := newServerFixture(t)

	err := f.client.Create(f.ctx, &v1alpha1.UIResource{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Status: v1alpha1.UIResourceStatus{
			Conditions: []v1alpha1.UIResourceCondition{
				{
					Type:               v1alpha1.UIResourceReady,
					Status:             metav1.ConditionTrue,
					LastTransitionTime: apis.NowMicro(),
				},
			},
		},
	})
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	streams := genericclioptions.IOStreams{Out: out}
	wait := newWaitCmd(streams)
	cmd := wait.register()

	err = cmd.Flags().Parse([]string{"--for=condition=Ready"})
	require.NoError(t, err)

	err = wait.run(f.ctx, []string{"uiresource/my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `uiresource.tilt.dev/my-sleep condition met`)
}
