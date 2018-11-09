package engine

import (
	"archive/tar"
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/wmclient/pkg/dirs"
)

func TestStaticDockerfileWithCache(t *testing.T) {
	f := newIBDFixture(t)
	defer f.TearDown()

	manifest := NewSanchoStaticManifest().WithCachePaths([]string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, manifest, store.BuildStateClean)
	assert.NoError(t, err)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3
LABEL "tilt.cache"="0"
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

func TestBaseDockerfileWithCache(t *testing.T) {
	f := newIBDFixture(t)
	defer f.TearDown()

	manifest := NewSanchoManifest().WithCachePaths([]string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, manifest, store.BuildStateClean)
	assert.NoError(t, err)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `FROM gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3
LABEL "tilt.cache"="0"
ADD . /
RUN ["go", "install", "github.com/windmilleng/sancho"]
ENTRYPOINT ["/go/bin/sancho"]
LABEL "tilt.buildMode"="scratch"`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.docker.BuildOptions.Context), expected)
}

type ibdFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	docker *docker.FakeDockerClient
	k8s    *k8s.FakeK8sClient
	ibd    *ImageBuildAndDeployer
}

func newIBDFixture(t *testing.T) *ibdFixture {
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewWindmillDirAt(f.Path())
	docker := docker.NewFakeDockerClient()
	ctx := output.CtxForTest()
	k8s := k8s.NewFakeK8sClient()
	ibd, err := provideImageBuildAndDeployer(ctx, docker, k8s, dir)
	if err != nil {
		t.Fatal(err)
	}
	return &ibdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		docker:         docker,
		k8s:            k8s,
		ibd:            ibd,
	}
}
