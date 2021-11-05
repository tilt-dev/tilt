//+build integration

package integration

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCustomDeploy(t *testing.T) {
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil || kubectlPath == "" {
		t.Fatal("`kubectl` not found in PATH")
	}

	f := newK8sFixture(t, "custom_deploy")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "someLabel=someValue1")

	// check that port forward is working
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:54871", "Welcome to nginx!")

	// check that pod log streaming is working (integration text fixture already streams logs to stdout,
	// so assert.True is used vs assert.Contains to avoid spewing a duplicate copy on failure)
	assert.True(t, strings.Contains(f.logs.String(), "start worker processes"),
		"Container logs not visible on stdout")

	// deploy.yaml is monitored by the FileWatch referenced by restartOn, so it should trigger
	// reconciliation and the KA Cmd should get re-invoked
	f.ReplaceContents("deploy.yaml", "someValue1", "someValue2")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "someLabel=someValue2")

	// verify port forward still works
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:54871", "Welcome to nginx!")

	// perform a Live Update (this version of the file can _only_ exist in the container via Live Update,
	// as it's overwriting a file from a public image)
	f.ReplaceContents(filepath.Join("web", "index.html"), "Hello", "Greetings")
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:54871", "Greetings from Live Update!")

	f.Touch(filepath.Join("web", "fallback.txt"))
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:54871", "Welcome to nginx!")
}
