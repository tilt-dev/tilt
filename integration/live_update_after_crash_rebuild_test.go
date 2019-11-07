//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLiveUpdateAfterCrashRebuild(t *testing.T) {
	f := newK8sFixture(t, "live_update_after_crash_rebuild")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	fmt.Println("> Waiting for pods from initial build")
	initialBuildPods := f.WaitForAllPodsReady(ctx, "app=live-update-after-crash-rebuild")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// Live update
	fmt.Println("> Perform a live update")
	f.ReplaceContents("compile.sh", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// Check that the pods were changed in place, and that we didn't create new ones
	afterLiveUpdatePods := f.WaitForAllPodsReady(ctx, "app=live-update-after-crash-rebuild")
	require.Equal(t, initialBuildPods, afterLiveUpdatePods)

	if len(afterLiveUpdatePods) != 1 {
		t.Fatalf("Expected one pod, actual: %v", afterLiveUpdatePods)
	}

	// Delete the pod and make sure it got replaced with something that prints the
	// same thing (crash rebuild).
	fmt.Println("> Kill pod, wait for crash rebuild")
	f.runCommandSilently("kubectl", "delete", "pod", afterLiveUpdatePods[0], namespaceFlag)

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// Unfortunately "WaitForAllPodsReady" actually checks for CONTAINER status,
	// and may pull in terminating pods (a container may be "ready" while its pod
	// is "terminating". We need to fix this, but in the meantime, sleep here to
	// increase the chance that containers are in the right state when we check.
	fmt.Println("> (Waiting for containers on dead pods to be !ready) ")
	time.Sleep(2 * time.Second)

	afterCrashRebuildPods := f.WaitForAllPodsReady(ctx, "app=live-update-after-crash-rebuild")
	require.NotEqual(t, afterLiveUpdatePods, afterCrashRebuildPods)

	// Another live update! Make sure that, after the crash rebuild, we're able to run more
	// live updates (i.e. that we have one and only one pod associated w/ the manifest)
	fmt.Println("> Perform another live update")
	f.ReplaceContents("compile.sh", "Two-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Three-Up! 🍄")

	afterSecondLiveUpdatePods := f.WaitForAllPodsReady(ctx, "app=live-update-after-crash-rebuild")

	// Check that the pods were changed in place, and that we didn't create new ones
	require.Equal(t, afterCrashRebuildPods, afterSecondLiveUpdatePods)
}
