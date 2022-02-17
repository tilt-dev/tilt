//go:build integration
// +build integration

package integration

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI_DockerPrune(t *testing.T) {
	f := newK8sFixture(t, "docker_prune")
	f.SetRestrictedCredentials()

	t.Log("Running `tilt ci` to trigger a build")
	f.TiltCI()
	t.Log("Running `tilt down` to stop running Pod so it can be pruned")
	var outBuf strings.Builder
	require.NoErrorf(t, f.tilt.Down(f.ctx, &outBuf),
		"Error running `tilt down`. Output:\n%s\n", &outBuf)

	outBuf.Reset()
	c := f.tilt.cmd(f.ctx, []string{"docker-prune", "--debug"}, io.MultiWriter(&outBuf, os.Stdout))
	t.Logf("Running command: %s", c.String())
	err := c.Run()
	require.NoError(t, err, "Error while running command")
	// use assert.True (instead of assert.Contains) - the cmd output is already
	// output for debugging purposes and assert.Contains produces duplicative
	// output (but hard to read due to embedded newlines)
	out := outBuf.String()
	assert.True(t,
		strings.Contains(out, "- untagged:"),
		"Image was not untagged")
	assert.True(t,
		strings.Contains(out, "- deleted: sha256:"),
		"Image was not deleted")
}
