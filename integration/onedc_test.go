//go:build integration
// +build integration

package integration

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOneDockerCompose(t *testing.T) {

	doV1V2(t, func(t *testing.T) {
		f := newDCFixture(t, "onedc")
		f.dockerKillAll("tilt")
		f.TiltUp()

		ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
		defer cancel()

		f.WaitUntil(ctx, "onedc up", func() (string, error) {
			name, err := f.dockerCmdOutput([]string{
				"ps", "-f", "name=onedc", "--format", "{{.Image}}",
			})
			// docker-compose-v1 uses underscores in the image name
			// docker compose v2 uses hyphens
			name = strings.Replace(name, "_", "-", -1)
			return name, err
		}, "onedc-web")

		f.CurlUntil(ctx, "onedc-web", "localhost:8000", "🍄 One-Up! 🍄")

		f.ReplaceContents("main.go", "One-Up", "Two-Up")

		ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
		defer cancel()
		f.CurlUntil(ctx, "onedc-web", "localhost:8000", "🍄 Two-Up! 🍄")
	})
}
