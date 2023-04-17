//go:build integration
// +build integration

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIgnores(t *testing.T) {
	f := newK8sFixture(t, "ignores")
	f.TiltUp()

	ctx, cancel := context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	_ = f.WaitForAllPodsReady(ctx, "app=ignores")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 One-Up! 🍄")

	f.ReplaceContents("ignored_by_tiltfile.txt", "ignored", "updated")
	f.ReplaceContents("ignored_by_dockerignore.txt", "ignored", "updated")
	f.ReplaceContents("ignored_by_tiltignore.txt", "ignored", "updated")
	f.ReplaceContents("compile.sh", "One-Up", "Two-Up")

	ctx, cancel = context.WithTimeout(f.ctx, time.Minute)
	defer cancel()
	f.CurlUntil(ctx, "http://localhost:31234", "🍄 Two-Up! 🍄")

	// The tiltignore'd file should be in the docker context, but
	// should not be synced.
	_, body, err := f.Curl("http://localhost:31234/ignored_by_tiltignore.txt")
	assert.NoError(t, err)
	assert.Contains(t, body, "should be ignored")

	// The dockerignore'd and tiltfile ignored file should not be in the docker context,
	// and not be synced.
	res, err := http.Get("http://localhost:31234/ignored_by_dockerignore.txt")
	assert.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusNotFound)

	res, err = http.Get("http://localhost:31234/ignored_by_tiltfile.txt")
	assert.NoError(t, err)
	assert.Equal(t, res.StatusCode, http.StatusNotFound)

	_, body, err = f.Curl("http://localhost:31234/topdir/subdir/src.txt")
	assert.NoError(t, err)
	assert.Contains(t, body, "hello")
}
