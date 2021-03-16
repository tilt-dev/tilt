//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

// Replacing a job often exercises very different codepaths
// than a deployment (because it is immutable)
func TestJob(t *testing.T) {
	f := newK8sFixture(t, "job")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234/message.txt", "🍄 One-Up! 🍄")

	f.ReplaceContents("message.txt", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234/message.txt", "🍄 Two-Up! 🍄")
}
