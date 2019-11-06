//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Assert that if we 'tilt up' in the same repo twice,
// it attaches to the existing pods without redeploying
func TestIdempotent(t *testing.T) {
	f := newK8sFixture(t, "idempotent")
	defer f.TearDown()

	f.TiltUp("idempotent")

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	firstPods := f.WaitForAllPodsReady(ctx, "app=idempotent")

	// Run it again, this time with a watch()
	f.TiltWatch()

	// Wait until the port-forwarder sets up
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "Idempotent One-Up!")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	secondPods := f.WaitForAllPodsReady(ctx, "app=idempotent")

	assert.Equal(t, firstPods, secondPods)
}
