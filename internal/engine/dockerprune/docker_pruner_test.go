package dockerprune

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/docker/go-units"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/testutils"
)

var (
	cachesPruned     = []string{"cacheA", "cacheB", "cacheC"}
	containersPruned = []string{"containerA", "containerB", "containerC"}
	numImages        = 3
	maxAge           = 11 * time.Hour
	ref              = container.MustParseNamed("some-ref")
)

var buildHistory = []model.BuildRecord{
	model.BuildRecord{StartTime: time.Now().Add(-24 * time.Hour)},
}

func twoHrsAgo() time.Time {
	return time.Now().Add(-2 * time.Hour)
}

func TestPruneFilters(t *testing.T) {
	f, imgSelectors := newFixture(t).withPruneOutput(cachesPruned, containersPruned, numImages)
	err := f.dp.prune(f.ctx, maxAge, imgSelectors)
	require.NoError(t, err)

	expectedFilters := filters.NewArgs(
		filters.Arg("label", docker.BuiltByTiltLabelStr),
		filters.Arg("until", maxAge.String()),
	)
	expectedImageFilters := filters.NewArgs(
		filters.Arg("label", docker.BuiltByTiltLabelStr),
	)

	assert.Equal(t, expectedFilters, f.dCli.BuildCachePruneOpts.Filters, "build cache prune filters")
	assert.Equal(t, expectedFilters, f.dCli.ContainersPruneFilters, "container prune filters")
	if assert.Len(t, f.dCli.ImageListOpts, 1, "expect exactly one call to ImageList") {
		assert.Equal(t, expectedImageFilters, f.dCli.ImageListOpts[0].Filters, "image list filters")
	}
}

func TestPruneOutput(t *testing.T) {
	f, imgSelectors := newFixture(t).withPruneOutput(cachesPruned, containersPruned, numImages)
	err := f.dp.prune(f.ctx, maxAge, imgSelectors)
	require.NoError(t, err)

	logs := f.logs.String()
	assert.Contains(t, logs, "[Docker Prune] removed 3 containers, reclaimed 3MB")
	assert.Contains(t, logs, "- containerC")
	assert.Contains(t, logs, "[Docker Prune] removed 3 images, reclaimed 6MB")
	assert.Contains(t, logs, "- deleted: build-id-2")
	assert.Contains(t, logs, "[Docker Prune] removed 3 caches, reclaimed 3MB")
	assert.Contains(t, logs, "- cacheC")
}

func TestPruneVersionTooLow(t *testing.T) {
	f, imgSelectors := newFixture(t).withPruneOutput(cachesPruned, containersPruned, numImages)
	f.dCli.ThrowNewVersionError = true
	err := f.dp.prune(f.ctx, maxAge, imgSelectors)
	require.NoError(t, err) // should log failure but not throw error

	logs := f.logs.String()
	assert.Contains(t, logs, "skipping Docker prune")

	// Should NOT have called any of the prune funcs
	assert.Empty(t, f.dCli.BuildCachePruneOpts)
	assert.Empty(t, f.dCli.ContainersPruneFilters)
	assert.Empty(t, f.dCli.ImageListOpts)
	assert.Empty(t, f.dCli.RemovedImageIDs)
}

func TestPruneSkipCachePruneIfVersionTooLow(t *testing.T) {
	f, imgSelectors := newFixture(t).withPruneOutput(cachesPruned, containersPruned, numImages)
	f.dCli.BuildCachePruneErr = f.dCli.VersionError("1.2.3", "build prune")
	err := f.dp.prune(f.ctx, maxAge, imgSelectors)
	require.NoError(t, err) // should log failure but not throw error

	logs := f.logs.String()
	assert.Contains(t, logs, "skipping build cache prune")

	// Should have called previous prune funcs as normal
	assert.NotEmpty(t, f.dCli.ContainersPruneFilters)
	assert.NotEmpty(t, f.dCli.ImageListOpts)
	assert.NotEmpty(t, f.dCli.RemovedImageIDs)
}

func TestPruneReturnsCachePruneError(t *testing.T) {
	f, imgSelectors := newFixture(t).withPruneOutput(cachesPruned, containersPruned, numImages)
	f.dCli.BuildCachePruneErr = fmt.Errorf("this is a real error, NOT an API version error")
	err := f.dp.prune(f.ctx, maxAge, imgSelectors)
	require.NotNil(t, err) // For all errors besides API version error, expect them to return
	assert.Contains(t, err.Error(), "this is a real error")

	logs := f.logs.String()
	assert.NotContains(t, logs, "skipping build cache prune")

	// Should have called previous prune funcs as normal
	assert.NotEmpty(t, f.dCli.ContainersPruneFilters)
	assert.NotEmpty(t, f.dCli.ImageListOpts)
	assert.NotEmpty(t, f.dCli.RemovedImageIDs)
}

func TestDeleteOldImages(t *testing.T) {
	f := newFixture(t)
	maxAge := 3 * time.Hour
	_, _ = f.withImageInspect(0, 25, time.Hour)      // young enough, won't be pruned
	_, ref := f.withImageInspect(1, 50, 4*time.Hour) // older than max age, will be pruned
	_, _ = f.withImageInspect(2, 75, 6*time.Hour)    // older than max age but doesn't match passed ref selectors
	report, err := f.dp.deleteOldImages(f.ctx, maxAge, []container.RefSelector{container.NameSelector(ref)})
	require.NoError(t, err)

	assert.Len(t, report.ImagesDeleted, 1, "expected exactly one deleted image")
	assert.Equal(t, 50, int(report.SpaceReclaimed), "expected space reclaimed")

	expectedDeleted := []string{"build-id-1"}
	assert.Equal(t, expectedDeleted, f.dCli.RemovedImageIDs)

	expectedFilters := filters.NewArgs(filters.Arg("label", docker.BuiltByTiltLabelStr))
	if assert.Len(t, f.dCli.ImageListOpts, 1, "expected exactly one call to ImageList") {
		assert.Equal(t, expectedFilters, f.dCli.ImageListOpts[0].Filters,
			"expected ImageList to called with label=builtby:tilt filter")
	}
}

func TestDeleteOldImagesDontRemoveImageWithMultipleTags(t *testing.T) {
	f := newFixture(t)
	maxAge := 3 * time.Hour
	id, ref := f.withImageInspect(0, 50, 4*time.Hour)
	inspect := f.dCli.Images[id]
	inspect.RepoTags = append(f.dCli.Images[id].RepoTags, "some-additional-tag")
	f.dCli.Images[id] = inspect

	report, err := f.dp.deleteOldImages(f.ctx, maxAge, []container.RefSelector{container.NameSelector(ref)})
	require.NoError(t, err) // error is silent

	assert.Len(t, report.ImagesDeleted, 0, "expected no deleted images")
	assert.Equal(t, 0, int(report.SpaceReclaimed), "expected space reclaimed")

	assert.Contains(t, f.logs.String(), "`docker image remove --force` required to remove an image with multiple tags")
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
	untilVals := f.dCli.ContainersPruneFilters.Get("until")
	require.Len(t, untilVals, 1, "unexpected number of filters for \"until\"")
	assert.Equal(t, untilVals[0], maxAge.String())
}

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

func (dpf *dockerPruneFixture) withPruneOutput(caches, containers []string, numImages int) (*dockerPruneFixture, []container.RefSelector) {
	dpf.dCli.BuildCachesPruned = caches
	dpf.dCli.ContainersPruned = containers

	selectors := make([]container.RefSelector, numImages)
	for i := 0; i < numImages; i++ {
		_, ref := dpf.withImageInspect(i, units.MB*(i+1), 48*time.Hour) // make each image 2 days old (def older than maxAge)
		selectors[i] = container.NameSelector(ref)
	}
	return dpf, selectors
}

func (dpf *dockerPruneFixture) withImageInspect(i, size int, timeSinceLastTag time.Duration) (id string, ref reference.Named) {
	id = fmt.Sprintf("build-id-%d", i)
	tag := fmt.Sprintf("%s-tag", id)
	dpf.dCli.Images[id] = types.ImageInspect{
		ID:       id,
		RepoTags: []string{tag},
		Size:     int64(size),
		Metadata: types.ImageMetadata{
			LastTagTime: time.Now().Add(-1 * timeSinceLastTag),
		},
	}
	dpf.dCli.ImageListCount += 1
	return id, container.MustParseNamed(tag)
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
			DeploymentRef: ref,
			BuildDetails:  model.DockerBuild{},
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
	store.CurrentlyBuilding[mn] = true
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
	// ContainerPrune was called -- we use this as a proxy for dp.Prune having been called.
	return dpf.dCli.ContainersPruneFilters.Len() > 0
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
