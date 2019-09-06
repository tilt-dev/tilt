//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveUpdateTwoImagesOneManifest(t *testing.T) {

	f := newK8sFixture(t, "live_update_two_images_one_manifest")
	defer f.TearDown()

	f.TiltWatch()

	sparkleURL := "http://localhost:8100"
	tadaURL := "http://localhost:8101"

	fmt.Println("> Initial build")
	initialPods := f.WaitForAllPodsReady(f.ctx, "app=twoimages")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, sparkleURL, "✨ One-Up! ✨\n")
	f.CurlUntil(ctx, tadaURL, "🎉 One-Up! 🎉\n")

	// Live Update only one
	fmt.Println("> LiveUpdate 'sparkle'")
	f.ReplaceContents("./sparkle/index.html", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, sparkleURL, "✨ Two-Up! ✨\n")
	f.CurlUntil(ctx, tadaURL, "🎉 One-Up! 🎉\n")

	podsAfterSparkleLiveUpd := f.WaitForAllPodsReady(ctx, "app=twoimages")

	// Assert that the pod was changed in-place / that we did NOT create new pod.
	assert.Equal(t, initialPods, podsAfterSparkleLiveUpd)

	// Kill the container we didn't LiveUpdate; k8s should quietly replace it, WITHOUT us
	// doing a crash rebuild (b/c that container didn't have state on it)
	// We expect the `kill` command to die abnormally when the parent process dies.
	fmt.Println("> kill 'tada' and wait for container to come back up")
	_, _ = f.runCommand("kubectl", "exec", podsAfterSparkleLiveUpd[0], "-c=tada", namespaceFlag,
		"--", "killall", "busybox")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, sparkleURL, "✨ Two-Up! ✨\n")
	f.CurlUntil(ctx, tadaURL, "🎉 One-Up! 🎉\n")

	podsAfterKillTada := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, initialPods, podsAfterKillTada)

	// Make sure that we can LiveUpdate both at once
	fmt.Println("> LiveUpdate both services at once")

	// NOTE(nick): There's a subtle race condition here. After we kill 'tada',
	// Tilt gets a pod event saying that the container died, and then a pod event
	// saying that the container came back. Because we don't have any insight into
	// the internal state of Tilt, we don't know when those events will show
	// up. If Tilt doesn't get them in time, it can't live update.
	//
	// For now, just sleep a couple seconds.
	time.Sleep(2 * time.Second)

	f.ReplaceContents("./sparkle/index.html", "Two-Up", "Three-Up")
	f.ReplaceContents("./tada/index.html", "One-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, sparkleURL, "✨ Three-Up! ✨\n")
	f.CurlUntil(ctx, tadaURL, "🎉 Three-Up! 🎉\n")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	podsAfterBothLiveUpdate := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.Equal(t, podsAfterKillTada, podsAfterBothLiveUpdate)

	// Kill a container we DID LiveUpdate; we should detect it and do a crash rebuild.
	fmt.Println("> kill 'sparkle' and wait for crash rebuild")
	_, _ = f.runCommand("kubectl", "exec", podsAfterBothLiveUpdate[0], "-c=sparkle", namespaceFlag,
		"--", "killall", "busybox")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, sparkleURL, "✨ Three-Up! ✨\n")
	f.CurlUntil(ctx, tadaURL, "🎉 Three-Up! 🎉\n")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	allPodsAfterKillSparkle := f.WaitForAllPodsReady(ctx, "app=twoimages")
	assert.NotEqual(t, podsAfterBothLiveUpdate, allPodsAfterKillSparkle)
}
