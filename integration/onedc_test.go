//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestOneDockerCompose(t *testing.T) {
	f := newDCFixture(t, "onedc")
	defer f.TearDown()

	f.dockerKillAll("tilt")
	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "onedc up", func() (string, error) {
		return f.dockerCmdOutput([]string{
			"ps", "-f", "name=onedc", "--format", "{{.Image}}",
		})
	}, "onedc")

	f.CurlUntil(ctx, "onedc", "localhost:8000", "🍄 One-Up! 🍄")

	f.ReplaceContents("main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "onedc", "localhost:8000", "🍄 Two-Up! 🍄")
}
