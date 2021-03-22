//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestOneUpCustom(t *testing.T) {
	f := newK8sFixture(t, "oneup_custom")
	defer f.TearDown()

	f.TiltCI("oneup-custom")

	// ForwardPort will fail if all the pods are not ready.
	//
	// We can't use the normal Tilt-managed forwards here because
	// Tilt doesn't setup forwards when --watch=false.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=oneup-custom")

	f.ForwardPort("deployment/oneup-custom", "31234:8000")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up Custom! 🍄")
}
