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

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/testutils"
)

var (
	cachesPruned     = []string{"cacheA", "cacheB", "cacheC"}
	containersPruned = []string{"containerA", "containerB", "containerC"}
	imagesPruned     = []string{"imageA", "imageB", "imageC"}
	maxAge           = 11 * time.Hour
)

var buildHistory = []model.BuildRecord{
	model.BuildRecord{StartTime: time.Now().Add(-24 * time.Hour)},
}

func twoHrsAgo() time.Time {
	return time.Now().Add(-2 * time.Hour)
}

func TestDockerPruneFilters(t *testing.T) {
	f := newFixture(t).withPruneOutput(cachesPruned, containersPruned, imagesPruned)
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
	f := newFixture(t).withPruneOutput(cachesPruned, containersPruned, imagesPruned)
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
	f := newFixture(t)
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
	f := newFixture(t)
	f.dCli.BuildCachePruneErr = f.dCli.VersionError("1.2.3", "build prune")
	err := f.dp.prune(f.ctx, maxAge)
	require.NoError(t, err) // should log failure but not throw error

	logs := f.logs.String()
	assert.Contains(t, logs, "skipping build cache prune")

	// Should have called the other prune funcs as normal
	assert.NotEmpty(t, f.dCli.ContainersPruneFilters)
	assert.NotEmpty(t, f.dCli.ImagesPruneFilters)
}

func TestDockerPruneReturnsCachePruneError(t *testing.T) {
	f := newFixture(t)
	f.dCli.BuildCachePruneErr = fmt.Errorf("this is a real error, NOT an API version error")
	err := f.dp.prune(f.ctx, maxAge) // For all errors besides API version error, expect them to return
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "this is a real error")

	logs := f.logs.String()
	assert.NotContains(t, logs, "skipping build cache prune")

	assert.NotEmpty(t, f.dCli.ImagesPruneFilters)  // called ImagesPrune before encountered error
	assert.Empty(t, f.dCli.ContainersPruneFilters) // should NOT have called any prune funcs AFTER CachePrune
}

func TestDockerPrunerSinceNBuilds(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withBuildCount(11)
	f.withDockerPruneSettings(true, 0, 5, 0)
	f.dp.lastPruneBuildCount = 5
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertPrune()
}

func TestDockerPrunerNotEnoughBuilds(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withBuildCount(11)
	f.withDockerPruneSettings(true, 0, 10, 0)
	f.dp.lastPruneBuildCount = 5
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerSinceInterval(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withDockerPruneSettings(true, 0, 0, 30*time.Minute)
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertPrune()
}

func TestDockerPrunerSinceDefaultInterval(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withDockerPruneSettings(true, 0, 0, 0)
	f.dp.lastPruneTime = time.Now().Add(-1 * (model.DockerPruneDefaultInterval + time.Minute))

	f.dp.OnChange(f.ctx, f.st)

	f.assertPrune()
}

func TestDockerPrunerNotEnoughTimeElapsed(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withDockerPruneSettings(true, 0, 0, 3*time.Hour)
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerSinceDefaultIntervalNotEnoughTime(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withDockerPruneSettings(true, 0, 0, 0)
	f.dp.lastPruneTime = time.Now().Add(-1 * model.DockerPruneDefaultInterval).Add(20 * time.Minute)

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerFirstRun(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withBuildCount(5)
	f.withDockerPruneSettings(true, 0, 10, 0)

	f.dp.OnChange(f.ctx, f.st)

	f.assertPrune()
}

func TestDockerPrunerFirstRunButNoCompletedBuilds(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withBuildCount(0)
	f.withDockerPruneSettings(true, 0, 10, 0)

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerNoDockerManifests(t *testing.T) {
	f := newFixture(t)
	f.withK8sOnlyManifest()
	f.withBuildCount(11)
	f.withDockerPruneSettings(true, 0, 5, 0)

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerDisabled(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withDockerPruneSettings(false, 0, 0, 0)

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerCurrentlyBuilding(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withCurrentlyBuilding("idk something")
	f.withDockerPruneSettings(true, 0, 0, time.Hour)
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerPendingBuild(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestUnbuilt() // manifest not yet built will be pending, so we should not prune
	f.withDockerPruneSettings(true, 0, 0, time.Hour)
	f.dp.lastPruneTime = twoHrsAgo()

	f.dp.OnChange(f.ctx, f.st)

	f.assertNoPrune()
}

func TestDockerPrunerMaxAgeFromSettings(t *testing.T) {
	f := newFixture(t)
	f.withDockerManifestAlreadyBuilt()
	f.withBuildCount(5)
	maxAge := time.Hour
	f.withDockerPruneSettings(true, maxAge, 10, 0)

	f.dp.OnChange(f.ctx, f.st)

	f.assertPrune()
	untilVals := f.dCli.ImagesPruneFilters.Get("until")
	require.Len(t, untilVals, 1, "unexpected number of filters for \"until\"")
	assert.Equal(t, untilVals[0], maxAge.String())
}

// Test currently building
type dockerPruneFixture struct {
	t    *testing.T
	ctx  context.Context
	logs *bytes.Buffer
	st   *store.Store

	dCli *docker.FakeClient
	dp   *DockerPruner
}

func newFixture(t *testing.T) *dockerPruneFixture {
	logs := new(bytes.Buffer)
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(logs)
	st, _ := store.NewStoreForTesting()

	dCli := docker.NewFakeClient()
	dp := NewDockerPruner(dCli)

	return &dockerPruneFixture{
		t:    t,
		ctx:  ctx,
		logs: logs,
		st:   st,
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

func (dpf *dockerPruneFixture) withDockerManifestAlreadyBuilt() {
	dpf.withDockerManifest(true)
}

func (dpf *dockerPruneFixture) withDockerManifestUnbuilt() {
	dpf.withDockerManifest(false)
}

func (dpf *dockerPruneFixture) withDockerManifest(alreadyBuilt bool) {
	m := model.Manifest{Name: "some-docker-manifest"}.WithImageTarget(
		model.ImageTarget{
			BuildDetails: model.DockerBuild{},
		})
	dpf.withManifestTarget(store.NewManifestTarget(m), alreadyBuilt)
}

func (dpf *dockerPruneFixture) withK8sOnlyManifest() {
	m := model.Manifest{Name: "i'm-k8s-only"}.WithDeployTarget(model.K8sTarget{})
	dpf.withManifestTarget(store.NewManifestTarget(m), true)
}

func (dpf *dockerPruneFixture) withManifestTarget(mt *store.ManifestTarget, alreadyBuilt bool) {
	if alreadyBuilt {
		// spoof build history so we think this manifest has already been built (i.e. isn't pending)
		mt.State.BuildHistory = buildHistory
	}

	store := dpf.st.LockMutableStateForTesting()
	store.UpsertManifestTarget(mt)
	dpf.st.UnlockMutableState()
}

func (dpf *dockerPruneFixture) withBuildCount(count int) {
	store := dpf.st.LockMutableStateForTesting()
	store.CompletedBuildCount = count
	dpf.st.UnlockMutableState()
}

func (dpf *dockerPruneFixture) withCurrentlyBuilding(mn model.ManifestName) {
	store := dpf.st.LockMutableStateForTesting()
	store.CurrentlyBuilding = mn
	dpf.st.UnlockMutableState()
}

func (dpf *dockerPruneFixture) withDockerPruneSettings(enabled bool, maxAge time.Duration, numBuilds int, interval time.Duration) {
	settings := model.DockerPruneSettings{
		Enabled:   enabled,
		MaxAge:    maxAge,
		NumBuilds: numBuilds,
		Interval:  interval,
	}
	store := dpf.st.LockMutableStateForTesting()
	store.DockerPruneSettings = settings
	dpf.st.UnlockMutableState()
}

func (dpf *dockerPruneFixture) pruneCalled() bool {
	// ImagePrune was called -- we use this as a proxy for dp.Prune having been called.
	return dpf.dCli.ImagesPruneFilters.Len() > 0
}

func (dpf *dockerPruneFixture) assertPrune() {
	if !dpf.pruneCalled() {
		dpf.t.Errorf("expected Prune() to be called, but it was not")
		dpf.t.FailNow()
	}
	if time.Since(dpf.dp.lastPruneTime) > time.Second {
		dpf.t.Errorf("Prune() was called, but dp.lastPruneTime was not updated/" +
			"not updated recently")
		dpf.t.FailNow()
	}
}

func (dpf *dockerPruneFixture) assertNoPrune() {
	if dpf.pruneCalled() {
		dpf.t.Errorf("Prune() was called, when no calls expected")
		dpf.t.FailNow()
	}
}
