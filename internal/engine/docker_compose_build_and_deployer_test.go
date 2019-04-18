package engine

import (
	"archive/tar"
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/wmclient/pkg/dirs"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

var expectedContainer = container.ID("dc-cont")
var confPath = "/whales/are/big/dc.yml"
var dcName = model.TargetName("MobyDick")
var dcTarg = model.DockerComposeTarget{Name: dcName, ConfigPath: confPath}

var imgRef = "gcr.io/some/image"
var imgTarg = model.NewImageTarget(container.MustParseSelector(imgRef)).
	WithBuildDetails(model.DockerBuild{
		Dockerfile: "Dockerfile.whales",
		BuildPath:  "/whales/are/big",
	})

func TestDockerComposeTargetBuilt(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	res, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{dcTarg}, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	if assert.Len(t, f.dcCli.UpCalls, 1, "expect one call to `docker-compose up`") {
		call := f.dcCli.UpCalls[0]
		assert.Equal(t, confPath, call.PathToConfig)
		assert.Equal(t, dcName, call.ServiceName)
		assert.True(t, call.ShouldBuild)
	}
	assert.Equal(t, expectedContainer, res.OneAndOnlyContainerID())
}

func TestTiltBuildsImage(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	res, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{imgTarg, dcTarg}, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount, "expect one docker build")

	expectedTag := fmt.Sprintf("%s:%s", imgRef, docker.TagLatest)
	assert.Equal(t, expectedTag, f.dCli.TagTarget)

	if assert.Len(t, f.dcCli.UpCalls, 1, "expect one call to `docker-compose up`") {
		call := f.dcCli.UpCalls[0]
		assert.Equal(t, confPath, call.PathToConfig)
		assert.Equal(t, dcName, call.ServiceName)
		assert.False(t, call.ShouldBuild, "should call `up` without `--build` b/c Tilt is doing the building")
	}

	assert.Len(t, res, 2, "expect two results (one for each spec)")
}

func TestTiltBuildsImageWithTag(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	refWithTag := "gcr.io/foo:bar"
	iTarget := model.NewImageTarget(container.MustParseSelector(refWithTag)).
		WithBuildDetails(model.DockerBuild{})

	_, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{iTarget, dcTarg}, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, refWithTag, f.dCli.TagTarget)
}

func TestDCBADRejectsAllSpecsIfOneUnsupported(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	specs := []model.TargetSpec{dcTarg, imgTarg, model.K8sTarget{}}

	iTarg, dcTarg := f.dcbad.extract(specs)
	assert.Empty(t, iTarg)
	assert.Empty(t, dcTarg)
}

func TestMultiStageDockerCompose(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f).
		WithDeployTarget(dcTarg)
	stateSet := store.BuildStateSet{}
	_, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.dCli.BuildCount)
	assert.Equal(t, 0, f.dCli.PushCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM docker.io/library/sancho-base:latest
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.dCli.BuildOptions.Context), expected)
}

func TestMultiStageDockerComposeWithOnlyOneDirtyImage(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f).
		WithDeployTarget(dcTarg)
	iTargetID := manifest.ImageTargets[0].ID()
	result := store.NewImageBuildResult(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	state := store.NewBuildState(result, nil)
	stateSet := store.BuildStateSet{iTargetID: state}
	_, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount)
	assert.Equal(t, 0, f.dCli.PushCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM docker.io/library/sancho-base:tilt-prebuilt
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.dCli.BuildOptions.Context), expected)
}

type dcbdFixture struct {
	*tempdir.TempDirFixture
	ctx   context.Context
	dcCli *dockercompose.FakeDCClient
	dCli  *docker.FakeClient
	dcbad *DockerComposeBuildAndDeployer
	st    *store.Store
}

func newDCBDFixture(t *testing.T) *dcbdFixture {
	ctx := output.CtxForTest()

	f := tempdir.NewTempDirFixture(t)

	dir := dirs.NewWindmillDirAt(f.Path())
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
	dcCli.ContainerIdOutput = expectedContainer
	dCli := docker.NewFakeClient()
	dcbad, err := provideDockerComposeBuildAndDeployer(ctx, dcCli, dCli, dir)
	if err != nil {
		t.Fatal(err)
	}
	st, _ := store.NewStoreForTesting()
	return &dcbdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		dcCli:          dcCli,
		dCli:           dCli,
		dcbad:          dcbad,
		st:             st,
	}
}
