//+build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestOneDockerCompose(t *testing.T) {
	f := newDCFixture(t, "onedc")
	defer f.TearDown()

	f.dockerKillAll("onedc_web")
	f.TiltWatch("web")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "onedc_web up", func() (string, error) {
		out := &bytes.Buffer{}
		cmd := f.dockerCmd([]string{
			"ps", "-f", "name=onedc_web", "--format", "{{.Image}}",
		}, out)
		err := cmd.Run()
		return out.String(), err
	}, "onedc_web")

	f.CurlUntil(ctx, "onedc_web", "localhost:8000", "🍄 One-Up! 🍄")

	f.ReplaceContents("main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "onedc_web", "localhost:8000", "🍄 Two-Up! 🍄")
}
