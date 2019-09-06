//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerComposeFastBuild(t *testing.T) {
	f := newDCFixture(t, "dcfastbuild")
	defer f.TearDown()

	f.dockerKillAll("tilt")
	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()

	f.WaitUntilContains(ctx, "dcfastbuild up", func() (string, error) {
		return f.dockerCmdOutput([]string{
			"ps", "-f", "name=dcfastbuild", "--format", "{{.Image}}",
		})
	}, "gcr.io/windmill-test-containers/dcfastbuild")

	createdAt := f.dockerCreatedAt("dcfastbuild")

	f.CurlUntil(ctx, "dcfastbuild", "localhost:8000", "🍄 One-Up! 🍄")

	f.ReplaceContents("cmd/dcfastbuild/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	f.CurlUntil(ctx, "dcfastbuild", "localhost:8000", "🍄 Two-Up! 🍄")

	createdAt2 := f.dockerCreatedAt("dcfastbuild")
	assert.Equal(t, createdAt, createdAt2,
		"Container restart detected. Expected container to be updated in-place")
}
