package build

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestCustomBuildSuccess(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true"}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), ref)
}

func TestCustomBuildSuccessSkipsLocalDocker(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", SkipsLocalDocker: true}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	assert.NoError(f.t, err)
	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-build-1551202573"), ref)
}

func TestCustomBuildCmdFails(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "false"}
	_, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	// TODO(dmiller) better error message
	assert.EqualError(t, err, "Custom build command failed: exit status 1")
}

func TestCustomBuildImgNotFound(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true"}
	_, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	assert.Contains(t, err.Error(), "fake docker client error: object not found")
}

func TestCustomBuildExpectedTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:the-tag"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: f.tdf.Path(), Command: "true", Tag: "the-tag"}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), ref)
}

func TestCustomBuilderExecsRelativeToTiltfile(t *testing.T) {
	f := newFakeCustomBuildFixture(t)
	defer f.teardown()

	f.tdf.WriteFile("proj/build.sh", "true")

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	cb := model.CustomBuild{WorkDir: filepath.Join(f.tdf.Path(), "proj"), Command: "./build.sh"}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), cb)
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), ref)
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

func (f *fakeCustomBuildFixture) teardown() {
	f.tdf.TearDown()
}
