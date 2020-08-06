//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRestartProcessDifferentUser(t *testing.T) {
	f := newK8sFixture(t, "restart_process_different_user")
	defer f.TearDown()

	f.TiltWatch()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	firstPods := f.WaitForAllPodsReady(ctx, "app=rpdu")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "🍄 One-Up! 🍄")
	f.CurlUntil(ctx, "http://localhost:8101", "🍄 One-Up! 🍄")

	f.ReplaceContents("source.txt", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8100", "🍄 Two-Up! 🍄")
	f.CurlUntil(ctx, "http://localhost:8101", "🍄 Two-Up! 🍄")

	secondPods := f.WaitForAllPodsReady(ctx, "app=rpdu")

	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, firstPods, secondPods)

	// Kill one of the containers, and make sure it gets replaced
	// We expect the `kill` command to die abnormally when the parent process dies.
	_, _ = f.runCommand("kubectl", "exec", secondPods[0], "-c=c2", namespaceFlag,
		"--", "kill", "1")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:8101", "🍄 Two-Up! 🍄")

	replacedPods := f.WaitForAllPodsReady(ctx, "app=rpdu")
	assert.NotEqual(t, secondPods, replacedPods)
}
