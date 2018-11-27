//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOneWatch(t *testing.T) {
	f := newFixture(t, "onewatch")
	defer f.TearDown()

	f.TiltWatch()

	// ForwardPort will fail if all the pods are not ready.
	// TODO(nick): We should make port-forwarding a primitive in the
	// Tiltfile since this seems generally useful, then get rid of this code.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	f.ForwardPort("deployment/onewatch", "31234:8000")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	f.ReplaceContents("main.go", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	twoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatch")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, oneUpPods, twoUpPods)
}
