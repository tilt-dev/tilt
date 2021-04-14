package cli

import (
	"bytes"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestEdit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip() // 'true' is not an editor on Windows
	}

	f := newServerFixture(t)
	defer f.TearDown()

	err := f.client.Create(f.ctx, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: "my-sleep"},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"sleep", "100"},
		},
	})
	require.NoError(t, err)

	out := bytes.NewBuffer(nil)
	cmd := newEditCmd()
	cmd.streams.ErrOut = out
	cmd.register()

	oldEditor := os.Getenv("EDITOR")
	defer os.Setenv("EDITOR", oldEditor)

	os.Setenv("EDITOR", "true")
	err = cmd.run(f.ctx, []string{"cmd", "my-sleep"})
	require.NoError(t, err)

	assert.Contains(t, out.String(), `Edit cancelled, no changes made`)
}
