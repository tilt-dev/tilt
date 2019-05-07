//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestWatchFalse(t *testing.T) {
	f := newK8sFixture(t, "watch_false")
	defer f.TearDown()

	// `tilt up --watch=false...` i.e. specifying no resource names;
	// should deploy ALL resources (in this case, two servers)
	f.TiltUp()

	// ForwardPort will fail if all the pods are not ready.
	//
	// We can't use the normal Tilt-managed forwards here because
	// Tilt doesn't setup forwards when --watch=false.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "test=watchfalse")

	f.ForwardPort("deployment/watchfalse1", "31234:8000")
	f.ForwardPort("deployment/watchfalse2", "31235:8000")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🚫👀")
	f.CurlUntil(ctx, "http://localhost:31235", "🚫👀")
}
