//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOneWatch(t *testing.T) {
	f := newK8sFixture(t, "onewatch")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// Introduce a syntax error
	f.ReplaceContents("compile.sh", "One-Up", "One-Up\"")

	f.WaitUntil(ctx, "live_update syntax error", func() (string, error) {
		return f.logs.String(), nil
	}, "FAILED TO UPDATE CONTAINER")

	f.ReplaceContents("compile.sh", "One-Up\"", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// Check that the pods were changed in place, and that we didn't create new ones
	twoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")
	require.Equal(t, oneUpPods, twoUpPods)

	if len(twoUpPods) != 1 {
		t.Fatalf("Expected one pod, actual: %v", twoUpPods)
	}

	// Delete the pod and make sure it got replaced with something that prints the
	// same thing (crash rebuild).
	f.runCommandSilently("kubectl", "delete", "pod", twoUpPods[0], namespaceFlag)

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// Unfortunately "WaitForAllPodsReady" isn't that accurate and can pull in terminating pods
	// too. Sleep here to increase the chance that pods are in the right state when we check.
	fmt.Println("> Waiting for dead pods to get into 'terminating' state")
	time.Sleep(2 * time.Second)

	newTwoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")
	require.NotEqual(t, twoUpPods, newTwoUpPods)

	// Another live update! Make sure that, after the crash rebuild, we're able to run more
	// live updates (i.e. that we have one and only one pod associated w/ the manifest)
	f.ReplaceContents("compile.sh", "Two-Up", "Three-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Three-Up! 🍄")

	threeUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	// Check that the pods were changed in place, and that we didn't create new ones
	require.Equal(t, newTwoUpPods, threeUpPods)
}
