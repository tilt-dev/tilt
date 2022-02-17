package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestDescribe(t *testing.T) {
	f := newServerFixture(t)

	err := f.client.Create(f.ctx, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"sleep", "1"},
		},
	})
	require.NoError(t, err)

	streams, _, out, _ := genericclioptions.NewTestIOStreams()
	describe := newDescribeCmd(streams)
	describe.register()

	err = describe.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `Name:         my-sleep`)
}
