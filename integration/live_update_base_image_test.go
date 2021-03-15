//+build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveUpdateBaseImage(t *testing.T) {
	f := newK8sFixture(t, "live_update_base_image")
	defer f.TearDown()

	f.TiltUp()

	timePerStage := time.Minute
	ctx, cancel := context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	firstBuild := f.WaitForAllPodsReady(ctx, "app=live-update-base-image")

	ctx, cancel = context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31000/message.txt", "Hello from regular")

	f.ReplaceContents("common/message.txt", "regular", "super unleaded")

	ctx, cancel = context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31000/message.txt", "Hello from super unleaded")

	secondBuild := f.WaitForAllPodsReady(ctx, "app=live-update-base-image")
	assert.Equal(t, firstBuild, secondBuild)
}
