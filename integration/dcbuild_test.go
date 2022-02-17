//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"
)

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

	f.CurlUntil(ctx, "dcbuild", "localhost:8000", "🍄 One-Up! 🍄")

	f.ReplaceContents("cmd/dcbuild/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "dcbuild", "localhost:8000", "🍄 Two-Up! 🍄")
}
