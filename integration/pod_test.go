//+build integration

package integration

import (
	"context"
	"testing"
	"time"
)

// Replacing a single pod often exercises very different codepaths
// than a deployment (because it has no owners and is immutable)
func TestPod(t *testing.T) {
	f := newK8sFixture(t, "pod")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234/message.txt", "🍄 One-Up! 🍄")

	f.ReplaceContents("message.txt", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234/message.txt", "🍄 Two-Up! 🍄")
}
