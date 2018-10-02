//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestOneUp(t *testing.T) {
	f := newFixture(t, "oneup")
	defer f.TearDown()

	f.TiltUp("oneup")
	f.ForwardPort("deployment/oneup", "31234:8000")

	ctx, cancel := context.WithTimeout(f.ctx, 20*time.Second)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")
}
