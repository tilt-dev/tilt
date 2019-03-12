//+build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestDockerComposeImageBuild(t *testing.T) {
	f := newDCFixture(t, "dcbuild")
	defer f.TearDown()

	f.dockerKillAll("dcbuild_web")
	f.TiltWatch("web")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()

	f.WaitUntil(ctx, "dcbuild_web up", func() (string, error) {
		out := &bytes.Buffer{}
		cmd := f.dockerCmd([]string{
			"ps", "-f", "name=dcbuild_web", "--format", "{{.Image}}",
		}, out)
		err := cmd.Run()
		return out.String(), err
	}, "gcr.io/windmill-test-containers/dcbuild")

	f.CurlUntil(ctx, "dcbuild_web", "localhost:8000", "🍄 One-Up! 🍄")
	if true {
		return
	}

	f.ReplaceContents("cmd/dcbuild/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "dcbuild_web", "localhost:8000", "🍄 Two-Up! 🍄")
}
