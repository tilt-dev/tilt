package build

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

var TwoURLRegistry = container.MustNewRegistryWithHostFromCluster("localhost:1234", "registry:1234")

func TestCustomBuildSuccess(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true"}
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefTaggedWithDigest(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["localhost:1234/foo_bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true"}
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("foo/bar", TwoURLRegistry), cb)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/foo_bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/foo_bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefWithCustomTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:my-tag"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", Tag: "my-tag"}
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("gcr.io/foo/bar", TwoURLRegistry), cb)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/gcr.io_foo_bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/gcr.io_foo_bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuildSuccessSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", SkipsLocalDocker: true}
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	require.NoError(f.t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-build-1551202573"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-build-1551202573"), refs.ClusterRef)
}

func TestCustomBuildSuccessClusterRefTaggedIfSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", SkipsLocalDocker: true}
	refs, err := f.cb.Build(f.ctx, refSetWithRegistryFromString("foo/bar", TwoURLRegistry), cb)
	require.NoError(f.t, err)

	assert.Equal(f.t, container.MustParseNamed("localhost:1234/foo_bar:tilt-build-1551202573"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("registry:1234/foo_bar:tilt-build-1551202573"), refs.ClusterRef)
}

func TestCustomBuildCmdFails(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "false"}
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	// TODO(dmiller) better error message
	assert.EqualError(t, err, "Custom build command failed: exit status 1")
}

func TestCustomBuildImgNotFound(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true"}
	_, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	assert.Contains(t, err.Error(), "fake docker client error: object not found")
}

func TestCustomBuildExpectedTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:the-tag"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", Tag: "the-tag"}
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	require.NoError(t, err)

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.ClusterRef)
}

func TestCustomBuilderExecsRelativeToTiltfile(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	f.tdf.WriteFile("proj/build.sh", "true")

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: filepath.Join(f.tdf.Path(), "proj"), Command: "./build.sh"}
	refs, err := f.cb.Build(f.ctx, refSetFromString("gcr.io/foo/bar"), cb)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), refs.LocalRef)
}

type fakeCustomBuildFixture struct {
	t    *testing.T
	ctx  context.Context
	dCli *docker.FakeClient
	cb   *ExecCustomBuilder
	tdf  *tempdir.TempDirFixture
}

func newFakeCustomBuildFixture(t *testing.T) *fakeCustomBuildFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dCli := docker.NewFakeClient()
	clock := fakeClock{
		now: time.Unix(1551202573, 0),
	}

	tdf := tempdir.NewTempDirFixture(t)

	cb := NewExecCustomBuilder(dCli, clock)

	f := &fakeCustomBuildFixture{
		t:    t,
		tdf:  tdf,
		ctx:  ctx,
		dCli: dCli,
		cb:   cb,
	}

	return f
}

func refSetFromString(s string) container.RefSet {
	sel := container.MustParseSelector(s)
	return container.MustSimpleRefSet(sel)
}

func refSetWithRegistryFromString(ref string, reg container.Registry) container.RefSet {
	r, err := container.NewRefSet(container.MustParseSelector(ref), reg)
	if err != nil {
		panic(err)
	}
	return r
}

func (f *fakeCustomBuildFixture) teardown() {
	f.tdf.TearDown()
}
