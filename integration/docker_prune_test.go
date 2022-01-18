//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLI_DockerPrune(t *testing.T) {
	f := newK8sFixture(t, "docker_prune")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltCI()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	c := f.tilt.cmd(ctx, []string{"docker-prune", "--debug"}, nil)
	t.Logf("Running command: %s", c.String())
	out, err := c.CombinedOutput()
	require.NoErrorf(t, err, "Error while running command. Output:\n%s\n", string(out))
	assert.Contains(t, string(out), "[Docker Prune] removed 2 caches")
}
