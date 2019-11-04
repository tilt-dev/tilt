//+build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWatchExec(t *testing.T) {
	f := newK8sFixture(t, "onewatch_exec")
	defer f.TearDown()

	f.TiltWatchExec()

	fmt.Printf("Wait for pods ready\n")
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	oneUpPods := f.WaitForAllPodsReady(ctx, "app=onewatchexec")

	fmt.Printf("Pods ready: %s\n", oneUpPods)
	fmt.Printf("Wait for server available on 31234\n")
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	fmt.Printf("Replace source.txt contents and wait for change to take effect")
	f.ReplaceContents("source.txt", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	twoUpPods := f.WaitForAllPodsReady(ctx, "app=onewatchexec")
	fmt.Printf("New pods ready: %s\n", twoUpPods)
	// Assert that the pods were changed in-place, and not that we
	// created new pods.
	assert.Equal(t, oneUpPods, twoUpPods)
}
