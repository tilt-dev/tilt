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
	initialPods := f.WaitForAllPodsReady(ctx, "app=twoimages")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ One-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	// Live Update only one
	f.ReplaceContents("./sparkle/main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ Two-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	podsAfterSparkleLiveUpd := f.WaitForAllPodsReady(ctx, "app=twoimages")

	// Assert that the pods were changed in-place / that we did NOT create new pods.
	assert.Equal(t, initialPods, podsAfterSparkleLiveUpd)

	// Kill the container we didn't LiveUpdate; k8s should quietly replace it, WITHOUT us
	// doing a crash rebuild (b/c that container didn't have state on it)
	// We expect the `kill` command to die abnormally when the parent process dies.
	_, _ = f.runCommand("kubectl", "exec", podsAfterSparkleLiveUpd[0], "-c=tada", namespaceFlag,
		"--", "kill", "1")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 One-Up! 🎉\n")

	podsAfterKillTada := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, podsAfterSparkleLiveUpd, podsAfterKillTada)

	// Kill the container we DID LiveUpdate; we should detect it and do a crash rebuild.
	_, _ = f.runCommand("kubectl", "exec", podsAfterKillTada[0], "-c=sparkle", namespaceFlag,
		"--", "kill", "1")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ Two-Up! ✨\n")

	podsAfterKillSparkle := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.NotEqual(t, podsAfterKillTada, podsAfterKillSparkle)

	// Make sure that we can LiveUpdate both at once
	f.ReplaceContents("./sparkle/main.go", "Two-Up", "Three-Up")
	f.ReplaceContents("./tada/main.go", "One-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "✨ Three-Up! ✨\n")
	f.CurlUntil(ctx, "http://localhost:8101", "🎉 Three-Up! 🎉\n")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	podsAfterLiveUpdBoth := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, podsAfterKillSparkle, podsAfterLiveUpdBoth)
}
