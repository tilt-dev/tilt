package engine

import (
	"archive/tar"
	"context"
	"strings"
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

	manifest := NewSanchoStaticManifestWithCache([]string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, manifest, store.BuildStateClean)
	if err != nil {
		t.Fatal(err)
	}

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

	manifest := NewSanchoFastBuildManifestWithCache(f, []string{"/root/.cache"})
	cache := "gcr.io/some-project-162817/sancho:tilt-cache-3de427a264f80719a58a9abd456487b3"
	f.docker.Images[cache] = types.ImageInspect{}

	_, err := f.ibd.BuildAndDeploy(f.ctx, manifest, store.BuildStateClean)
	if err != nil {
		t.Fatal(err)
	}

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

func TestDeployTwinImages(t *testing.T) {
	f := newIBDFixture(t)
	defer f.TearDown()

	sancho := NewSanchoFastBuildManifest(f)
	manifest := sancho.WithDeployInfo(sancho.K8sInfo().AppendYAML(SanchoTwinYAML))
	result, err := f.ibd.BuildAndDeploy(f.ctx, manifest, store.BuildStateClean)
	if err != nil {
		t.Fatal(err)
	}

	expectedImage := "gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	assert.Equal(t, expectedImage, result.Image.String())
	assert.Equalf(t, 2, strings.Count(f.k8s.Yaml, expectedImage),
		"Expected image to update twice in YAML: %s", f.k8s.Yaml)
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
