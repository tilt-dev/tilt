//+build integration

package integration

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLiveUpdateOnly(t *testing.T) {
	f := newK8sFixture(t, "live_update_only")
	defer f.TearDown()
	f.SetRestrictedCredentials()

	f.TiltUp()

	// ForwardPort will fail if all the pods are not ready.
	//
	// We can't use the normal Tilt-managed forwards here because
	// Tilt doesn't setup forwards when --watch=false.
	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.WaitForAllPodsReady(ctx, "app=lu-only")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	// since we're using a public image, until we modify a file, the original contents are there
	f.CurlUntil(ctx, "http://localhost:28195", "Welcome to nginx!")

	f.ReplaceContents(filepath.Join("web", "index.html"), "Hello", "Greetings")

	// verify file was changed (we know it's the same pod because this file can ONLY exist via Live Update sync)
	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:28195", "Greetings from Live Update!")

	f.ReplaceContents("special.txt",
		"this file triggers a full rebuild",
		"time to rebuild!")

	// TODO(milas): this is ridiculously hacky - we should use `tilt wait` once that exists
	// 	or otherwise poll the API manually instead of playing with log strings
	var logs strings.Builder
	assert.Eventually(t, func() bool {
		logs.WriteString(f.logs.String())

		logStr := logs.String()
		fileChangeIdx := strings.Index(logStr, `1 File Changed: [special.txt]`)
		if fileChangeIdx == -1 {
			return false
		}

		afterFileChangeLogs := logStr[fileChangeIdx:]
		return strings.Contains(afterFileChangeLogs, `STEP 1/1 — Deploying`)
	}, 15*time.Second, 500*time.Millisecond, "Full rebuild never triggered")

	// attempt another Live Update
	f.ReplaceContents(filepath.Join("web", "index.html"), "Greetings", "Salutations")
	ctx, cancel = context.WithTimeout(f.ctx, 10*time.Second)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:28195", "Salutations from Live Update!")
}
