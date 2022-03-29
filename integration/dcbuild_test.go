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

// Ensures live-update works on tilt-handled image builds in dockercompose
func TestDockerComposeImageBuild(t *testing.T) {
	f := newDCFixture(t, "dcbuild")

	f.dockerKillAll("tilt")
	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "dcbuild up", func() (string, error) {
		return f.dockerCmdOutput([]string{
			"ps", "-f", "name=dcbuild", "--format", "{{.Image}}",
		})
	}, "gcr.io/windmill-test-containers/dcbuild")

	f.CurlUntil(ctx, "dcbuild", "localhost:8000/index.html", "🍄 One-Up! 🍄")

	cID1, err := f.dockerContainerID("dcbuild")
	require.NoError(t, err)

	f.ReplaceContents("compile.sh", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "dcbuild", "localhost:8000/index.html", "🍄 Two-Up! 🍄")

	cID2, err := f.dockerContainerID("dcbuild")
	require.NoError(t, err)

	// Make sure the container was updated in-place
	assert.Equal(t, cID1, cID2)
}
