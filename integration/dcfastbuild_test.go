//+build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerComposeFastBuild(t *testing.T) {
	f := newDCFixture(t, "dcfastbuild")
	defer f.TearDown()

	f.dockerKillAll("dcfastbuild_web")
	f.TiltWatch("web")

	ctx, cancel := context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()

	f.WaitUntil(ctx, "dcfastbuild_web up", func() (string, error) {
		out := &bytes.Buffer{}
		cmd := f.dockerCmd([]string{
			"ps", "-f", "name=dcfastbuild_web", "--format", "{{.Image}}",
		}, out)
		err := cmd.Run()
		return out.String(), err
	}, "gcr.io/windmill-test-containers/dcfastbuild")

	createdAt := f.dockerCreatedAt("dcfastbuild_web")

	f.CurlUntil(ctx, "dcfastbuild_web", "localhost:8000", "🍄 One-Up! 🍄")

	f.ReplaceContents("cmd/dcfastbuild/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, 30*time.Second)
	defer cancel()
	f.CurlUntil(ctx, "dcfastbuild_web", "localhost:8000", "🍄 Two-Up! 🍄")

	createdAt2 := f.dockerCreatedAt("dcfastbuild_web")
	assert.Equal(t, createdAt, createdAt2, "Container restarted changed in fast build")
}
