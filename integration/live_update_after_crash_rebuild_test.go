//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/require"
)

func TestLiveUpdateAfterCrashRebuild(t *testing.T) {
	// TODO(maia): investigate unexplained errors in CI, reenable
	// e.g.: https://circleci.com/gh/windmilleng/tilt/28109
	t.SkipNow()

	f := newK8sFixture(t, "live_update_after_crash_rebuild")
	defer f.TearDown()

	f.SetRestrictedCredentials()

	pw := f.newPodWaiter("app=live-update-after-crash-rebuild").
		withExpectedPhase(v1.PodRunning)
	initialPods := pw.wait()

	f.TiltWatch()

	fmt.Println("> Waiting for pods from initial build")

	pw = pw.withExpectedPodCount(1)

	initialBuildPods := pw.withDisallowedPodIDs(initialPods).wait()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// Live update
	fmt.Println("> Perform a live update")
	f.ReplaceContents("compile.sh", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// Check that the pods were changed in place, and that we didn't create new ones
	afterLiveUpdatePods := pw.withDisallowedPodIDs(initialPods).wait()
	require.Equal(t, initialBuildPods, afterLiveUpdatePods, "after first live update")

	// Delete the pod and make sure it got replaced with something that prints the
	// same thing (crash rebuild).
	fmt.Println("> Kill pod, wait for crash rebuild")
	f.runCommandSilently("kubectl", "delete", "pod", afterLiveUpdatePods[0], namespaceFlag)

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	afterCrashRebuildPods := pw.withDisallowedPodIDs(afterLiveUpdatePods).wait()

	// Another live update! Make sure that, after the crash rebuild, we're able to run more
	// live updates (i.e. that we have one and only one pod associated w/ the manifest)
	fmt.Println("> Perform another live update")
	f.ReplaceContents("compile.sh", "Two-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Three-Up! 🍄")

	afterSecondLiveUpdatePods := pw.withDisallowedPodIDs(afterLiveUpdatePods).wait()

	// Check that the pods were changed in place, and that we didn't create new ones
	require.Equal(t, afterCrashRebuildPods, afterSecondLiveUpdatePods)
}
