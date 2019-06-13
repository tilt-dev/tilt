//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWatchExec(t *testing.T) {
	f := newK8sFixture(t, "onewatch_exec")
	defer f.TearDown()

	f.TiltWatchExec()

	// ForwardPort will fail if all the pods are not ready.
	//
	// For the purposes of this integration tests, we want the test to fail if the
	// Pod is re-created (rather than getting updated in-place).  We deliberately
	// don't use Tilt-managed port-forwarding because it would auto-connect to the
	// new pod.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=onewatchexec")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	f.ReplaceContents("app.py", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	twoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatchexec")
	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, oneUpPods, twoUpPods)

}
