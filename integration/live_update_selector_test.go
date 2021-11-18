//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveUpdateSelector(t *testing.T) {
	f := newK8sFixture(t, "live_update_selector")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=live-update-selector")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// Introduce a syntax error
	f.ReplaceContents("compile.sh", "One-Up", "One-Up\"")

	f.WaitUntil(ctx, "live_update syntax error", func() (string, error) {
		return f.logs.String(), nil
	}, "Failed to update container")

	f.ReplaceContents("compile.sh", "One-Up\"", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	twoUpPods := f.WaitForAllPodsReady(ctx, "app=live-update-selector")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, oneUpPods, twoUpPods)

	if len(twoUpPods) != 1 {
		t.Fatalf("Expected one pod, actual: %v", twoUpPods)
	}

	// Delete the pod and make sure it got replaced with something that prints the
	// same thing (crash rebuild).
	f.runCommandSilently("kubectl", "delete", "pod", twoUpPods[0], namespaceFlag)

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	newTwoUpPods := f.WaitForAllPodsReady(ctx, "app=live-update-selector")
	assert.NotEqual(t, twoUpPods, newTwoUpPods)
}
