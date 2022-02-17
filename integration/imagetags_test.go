//go:build integration
// +build integration

package integration

import (
	"context"
	"testing"
	"time"
)

func TestImageTags(t *testing.T) {
	f := newK8sFixture(t, "imagetags")

	f.TiltUp()

	timePerStage := time.Minute
	ctx, cancel := context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=imagetags")
	f.WaitForAllPodsReady(ctx, "app=imagetags-stretch")

	ctx, cancel = context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31000/message.txt", "Hello from regular")
	f.CurlUntil(ctx, "http://localhost:31001/message.txt", "Hello from stretch")

	f.ReplaceContents("common/message.txt", "regular", "super unleaded")

	ctx, cancel = context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31000/message.txt", "Hello from super unleaded")
	f.CurlUntil(ctx, "http://localhost:31001/message.txt", "Hello from stretch")

	f.ReplaceContents("common-stretch/message.txt", "stretch", "armstrong")

	ctx, cancel = context.WithTimeout(f.ctx, timePerStage)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31001/message.txt", "Hello from armstrong")
	f.CurlUntil(ctx, "http://localhost:31000/message.txt", "Hello from super unleaded")
}
