//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOneWatch(t *testing.T) {
	f := newK8sFixture(t, "onewatch")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltWatch()

	// ForwardPort will fail if all the pods are not ready.
	//
	// For the purposes of this integration tests, we want the test to fail if the
	// Pod is re-created (rather than getting updated in-place).  We deliberately
	// don't use Tilt-managed port-forwarding because it would auto-connect to the
	// new pod.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	f.ForwardPort("deployment/onewatch", "31234:8000")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// Introduce a syntax error
	f.ReplaceContents("main.go", "One-Up", "One-Up\"")

	f.WaitUntil(ctx, "live_update syntax error", func() (string, error) {
		return f.logs.String(), nil
	}, "FAILED TO UPDATE CONTAINER")

	f.ReplaceContents("main.go", "One-Up\"", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	twoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, oneUpPods, twoUpPods)
}
