//+build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"
)

func TestOneDockerCompose(t *testing.T) {
	f := newFixture(t, "onedc")
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

	cID := f.dockerContainerID("onedc_web")

	f.WaitUntil(ctx, fmt.Sprintf("onedc_web curl(%s)", cID), func() (string, error) {
		out := &bytes.Buffer{}
		cmd := f.dockerCmd([]string{
			"exec", cID, "curl", "-s", "localhost:8000",
		}, out)
		err := cmd.Run()
		return out.String(), err
	}, "🍄 One-Up! 🍄")

	// TODO(nick): uncomment when file-watching works
	// f.ReplaceContents("main.go", "One-Up", "Two-Up")

	// ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	// defer cancel()
	// f.CurlUntil(ctx, "http://localhost:31235", "🍄 Two-Up! 🍄")
}
