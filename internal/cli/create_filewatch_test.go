package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreateFileWatch(t *testing.T) {
	f := newServerFixture(t)
	defer f.TearDown()

	out := bytes.NewBuffer(nil)

	cmd := newCreateFileWatchCmd()
	cmd.helper.streams.Out = out
	c := cmd.register()
	err := c.Flags().Parse([]string{
		"--ignore", "web/node_modules",
	})
	require.NoError(t, err)

	err = cmd.run(f.ctx, []string{"my-watch", "src", "web"})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `filewatch.tilt.dev/my-watch created`)

	var fw v1alpha1.FileWatch
	err = f.client.Get(f.ctx, types.NamespacedName{Name: "my-watch"}, &fw)
	require.NoError(t, err)

	cwd, _ := os.Getwd()
	assert.Equal(t, []string{
		filepath.Join(cwd, "src"),
		filepath.Join(cwd, "web"),
	}, fw.Spec.WatchedPaths)
	assert.Equal(t, cwd, fw.Spec.Ignores[0].BasePath)
	assert.Equal(t, []string{"web/node_modules"}, fw.Spec.Ignores[0].Patterns)
}
