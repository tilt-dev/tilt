//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveUpdateTwoImagesOneManifest(t *testing.T) {
	f := newK8sFixture(t, "live_update_two_images_one_manifest")
	defer f.TearDown()

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	firstPods := f.WaitForAllPodsReady(ctx, "app=twoimages")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ One-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	f.ReplaceContents("./sparkle/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ Two-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	secondPods := f.WaitForAllPodsReady(ctx, "app=twoimages")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, firstPods, secondPods)

	// Kill the container we didn't LiveUpdate; k8s should quietly replace it, WITHOUT us
	// doing a crash rebuild (b/c that container didn't have state on it)
	// We expect the `kill` command to die abnormally when the parent process dies.
	_, _ = f.runCommand("kubectl", "exec", secondPods[0], "-c=tada", namespaceFlag,
		"--", "kill", "1")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	thirdPods := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, secondPods, thirdPods)

	// Make sure that we can LiveUpdate both at once
	f.ReplaceContents("./sparkle/main.go", "Two-Up", "Three-Up")
	f.ReplaceContents("./tada/main.go", "One-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	fourthPods := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, thirdPods, fourthPods)

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ Three-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 Three-Up! 🎉\n")
}
