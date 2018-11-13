package build

import (
	"archive/tar"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestCacheBuilderRef(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	ref := container.MustParseNamedTagged("gcr.io/nicks/image:source")
	paths := []string{"/src/node_modules", "/src/yarn.lock"}
	cacheRef, err := f.cb.cacheRef(ref, paths)
	assert.NoError(t, err)
	assert.Equal(t, "gcr.io/nicks/image:tilt-cache-1a9aa4aa0297919d6a59e8ee15eb9f6b", cacheRef.String())
}

func TestCacheBuilder(t *testing.T) {
	f := newFakeDockerBuildFixture(t)
	defer f.teardown()

	ref := container.MustParseNamedTagged("gcr.io/nicks/image:source")
	paths := []string{"/src/node_modules", "/src/yarn.lock"}
	df := dockerfile.Dockerfile("FROM golang:10")
	err := f.cb.CreateCacheFrom(f.ctx, df, ref, paths, model.DockerArgs{})
	assert.NoError(t, err)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/nicks/image:source as tilt-source
FROM golang:10
COPY --from=tilt-source /src/node_modules /src/node_modules
COPY --from=tilt-source /src/yarn.lock /src/yarn.lock
LABEL "tilt.cache"="1"`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.fakeDocker.BuildOptions.Context), expected)
}
