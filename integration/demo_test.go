//+build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestTiltDemo(t *testing.T) {
	f := newFixture(t, "")
	f.skipTiltDown = true
	defer f.TearDown()

	f.TiltDemo()

	ctx, cancel := context.WithTimeout(f.ctx, 3*time.Minute)
	defer cancel()

	f.CurlUntilStatusCode(ctx, "http://localhost:5734/ready", http.StatusNoContent)
}
