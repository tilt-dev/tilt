//go:build integration
// +build integration

package integration

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOneUp(t *testing.T) {
	f := newK8sFixture(t, "oneup")
	f.SetRestrictedCredentials()

	f.TiltUp("oneup")

	// ForwardPort will fail if all the pods are not ready.
	//
	// We can't use the normal Tilt-managed forwards here because
	// Tilt doesn't setup forwards when --watch=false.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=oneup")

	f.ForwardPort("deployment/oneup", "31234:8000")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	// minimal sanity check that the engine dump works - this really just ensures that there's no egregious
	// serialization issues
	var b bytes.Buffer
	assert.NoErrorf(t, f.tilt.DumpEngine(f.ctx, &b), "Failed to dump engine state, command output:\n%s", b.String())
}
