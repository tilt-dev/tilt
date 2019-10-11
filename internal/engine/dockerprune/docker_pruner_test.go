package dockerprune

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/testutils"
)

var (
	cachesPruned     = []string{"cacheA", "cacheB", "cacheC"}
	containersPruned = []string{"containerA", "containerB", "containerC"}
	imagesPruned     = []string{"imageA", "imageB", "imageC"}
	maxAge           = 11 * time.Hour
)

func TestDockerPruneFilters(t *testing.T) {
	f := newFixture().withPruneOutput(cachesPruned, containersPruned, imagesPruned)
	err := f.dp.prune(f.ctx, maxAge)
	require.NoError(t, err)

	expectedFilters := filters.NewArgs(
		filters.Arg("label", docker.BuiltByTiltLabelStr),
		filters.Arg("until", maxAge.String()),
	)
	expectedImageFilters := filters.NewArgs(
		filters.Arg("label", docker.BuiltByTiltLabelStr),
		filters.Arg("until", maxAge.String()),
		filters.Arg("dangling", "0"),
	)

	assert.Equal(t, expectedFilters, f.dCli.BuildCachePruneOpts.Filters, "build cache prune filters")
	assert.Equal(t, expectedFilters, f.dCli.ContainersPruneFilters, "container prune filters")
	assert.Equal(t, expectedImageFilters, f.dCli.ImagesPruneFilters, "image prune filters")
}

func TestDockerPruneOutput(t *testing.T) {
	f := newFixture().withPruneOutput(cachesPruned, containersPruned, imagesPruned)
	err := f.dp.prune(f.ctx, maxAge)
	require.NoError(t, err)

	logs := f.logs.String()
	assert.Contains(t, logs, "[Docker Prune] removed 3 caches, reclaimed 3 bytes")
	assert.Contains(t, logs, "- cacheC")
	assert.Contains(t, logs, "[Docker Prune] removed 3 containers, reclaimed 3 bytes")
	assert.Contains(t, logs, "- containerC")
	assert.Contains(t, logs, "[Docker Prune] removed 3 images, reclaimed 3 bytes")
	assert.Contains(t, logs, "- deleted: imageC")
}

func TestDockerPruneVersionTooLow(t *testing.T) {
	f := newFixture()
	f.dCli.ThrowNewVersionError = true
	err := f.dp.prune(f.ctx, maxAge)
	require.NoError(t, err) // should log failure but not throw error

	logs := f.logs.String()
	assert.Contains(t, logs, "skipping Docker prune")

	// Should NOT have called any of the prune funcs
	assert.Empty(t, f.dCli.BuildCachePruneOpts)
	assert.Empty(t, f.dCli.ContainersPruneFilters)
	assert.Empty(t, f.dCli.ImagesPruneFilters)
}

func TestDockerPruneSkipCachePruneIfVersionTooLow(t *testing.T) {
	f := newFixture()
	f.dCli.BuildCachePruneErr = f.dCli.VersionError("1.2.3", "build prune")
	err := f.dp.prune(f.ctx, maxAge)
	require.NoError(t, err) // should log failure but not throw error

	logs := f.logs.String()
	assert.Contains(t, logs, "skipping build cache prune")

	// Should have called subsequent prune funcs as normal
	assert.NotEmpty(t, f.dCli.ContainersPruneFilters)
	assert.NotEmpty(t, f.dCli.ImagesPruneFilters)
}

func TestDockerPruneReturnsCachePruneError(t *testing.T) {
	f := newFixture()
	f.dCli.BuildCachePruneErr = fmt.Errorf("this is a real error, NOT an API version error")
	err := f.dp.prune(f.ctx, maxAge) // For all errors besides API version error, expect them to return
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "this is a real error")

	logs := f.logs.String()
	assert.NotContains(t, logs, "skipping build cache prune")

	// Should NOT have called any subsequent prune funcs
	assert.Empty(t, f.dCli.ContainersPruneFilters)
	assert.Empty(t, f.dCli.ImagesPruneFilters)
}

type dockerPruneFixture struct {
	ctx  context.Context
	logs *bytes.Buffer

	dCli *docker.FakeClient
	dp   *DockerPruner
}

func newFixture() *dockerPruneFixture {
	logs := new(bytes.Buffer)
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(logs)

	dCli := docker.NewFakeClient()
	dp := NewDockerPruner(dCli)

	return &dockerPruneFixture{
		ctx:  ctx,
		logs: logs,
		dCli: dCli,
		dp:   dp,
	}
}

func (dpf *dockerPruneFixture) withPruneOutput(caches, containers, images []string) *dockerPruneFixture {
	dpf.dCli.BuildCachesPruned = caches
	dpf.dCli.ContainersPruned = containers
	dpf.dCli.ImagesPruned = images
	return dpf
}
