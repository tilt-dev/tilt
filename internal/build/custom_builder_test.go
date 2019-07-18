package build

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
)

func TestCustomBuildSuccess(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), "true", "")
	if err != nil {
		f.t.Fatal(err)
	}

	assert.Equal(f.t, container.MustParseNamed("gcr.io/foo/bar:tilt-11cd0eb38bc3ceb9"), ref)
}

func TestCustomBuildCmdFails(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	_, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), "false", "")
	// TODO(dmiller) better error message
	assert.EqualError(t, err, "exit status 1")
}

func TestCustomBuildImgNotFound(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	_, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), "true", "")
	assert.Contains(t, err.Error(), "fake docker client error: object not found")
}

func TestCustomBuildExpectedTag(t *testing.T) {
	f := newFakeCustomBuildFixture(t)

	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.dCli.Images["gcr.io/foo/bar:the-tag"] = types.ImageInspect{ID: string(sha)}
	ref, err := f.cb.Build(f.ctx, container.MustParseNamed("gcr.io/foo/bar"), "true", "the-tag")
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
}

func newFakeCustomBuildFixture(t *testing.T) *fakeCustomBuildFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	dCli := docker.NewFakeClient()
	clock := fakeClock{
		now: time.Unix(1551202573, 0),
	}

	cb := NewExecCustomBuilder(dCli, clock)

	f := &fakeCustomBuildFixture{
		t:    t,
		ctx:  ctx,
		dCli: dCli,
		cb:   cb,
	}

	return f
}
