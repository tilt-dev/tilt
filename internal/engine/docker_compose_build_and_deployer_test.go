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
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestDockerComposeTargetBuilt(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	expectedContainerID := "fake-container-id"
	f.dcCli.ContainerIdOutput = container.ID(expectedContainerID)

	manifest := manifestbuilder.New(f, "fe").WithDockerCompose().Build()
	dcTarg := manifest.DockerComposeTarget()

	res, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	if assert.Len(t, f.dcCli.UpCalls, 1, "expect one call to `docker-compose up`") {
		call := f.dcCli.UpCalls[0]
		assert.Equal(t, dcTarg.ConfigPaths, call.PathToConfig)
		assert.Equal(t, "fe", call.ServiceName.String())
		assert.True(t, call.ShouldBuild)
	}

	dRes := res[dcTarg.ID()].(store.DockerComposeBuildResult)
	assert.Equal(t, expectedContainerID, dRes.DockerComposeContainerID.String())
}

func TestTiltBuildsImage(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	iTarget := NewSanchoDockerBuildImageTarget(f)
	manifest := manifestbuilder.New(f, "fe").
		WithDockerCompose().
		WithImageTarget(iTarget).
		Build()
	dcTarg := manifest.DockerComposeTarget()

	res, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount, "expect one docker build")

	expectedTag := fmt.Sprintf("%s:%s", iTarget.Refs.LocalRef(), docker.TagLatest)
	assert.Equal(t, expectedTag, f.dCli.TagTarget)

	if assert.Len(t, f.dcCli.UpCalls, 1, "expect one call to `docker-compose up`") {
		call := f.dcCli.UpCalls[0]
		assert.Equal(t, dcTarg.ConfigPaths, call.PathToConfig)
		assert.Equal(t, "fe", call.ServiceName.String())
		assert.False(t, call.ShouldBuild, "should call `up` without `--build` b/c Tilt is doing the building")
	}

	assert.Len(t, res, 2, "expect two results (one for each spec)")
}

func TestTiltBuildsImageWithTag(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	refWithTag := "gcr.io/foo:bar"
	iTarget := model.MustNewImageTarget(container.MustParseSelector(refWithTag)).
		WithBuildDetails(model.DockerBuild{})
	manifest := manifestbuilder.New(f, "fe").
		WithDockerCompose().
		WithImageTarget(iTarget).
		Build()

	_, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, refWithTag, f.dCli.TagTarget)
}

func TestDCBADRejectsAllSpecsIfOneUnsupported(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	specs := []model.TargetSpec{model.DockerComposeTarget{}, model.ImageTarget{}, model.K8sTarget{}}

	iTarg, dcTarg := f.dcbad.extract(specs)
	assert.Empty(t, iTarg)
	assert.Empty(t, dcTarg)
}

func TestMultiStageDockerCompose(t *testing.T) {
	f := newDCBDFixture(t)
	defer f.TearDown()

	manifest := NewSanchoDockerBuildMultiStageManifest(f).
		WithDeployTarget(defaultDockerComposeTarget(f, "sancho"))

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
FROM sancho-base:latest
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
		WithDeployTarget(defaultDockerComposeTarget(f, "sancho"))

	iTargetID := manifest.ImageTargets[0].ID()
	result := store.NewImageBuildResultSingleRef(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	state := store.NewBuildState(result, nil)
	stateSet := store.BuildStateSet{iTargetID: state}
	f.dCli.ImageListCount = 1
	_, err := f.dcbad.BuildAndDeploy(f.ctx, f.st, buildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount)
	assert.Equal(t, 0, f.dCli.PushCount)

	expected := expectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-prebuilt
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
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	f := tempdir.NewTempDirFixture(t)

	dir := dirs.NewWindmillDirAt(f.Path())
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
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

func defaultDockerComposeTarget(f Fixture, name string) model.DockerComposeTarget {
	return model.DockerComposeTarget{
		Name:        model.TargetName(name),
		ConfigPaths: []string{f.JoinPath("docker-compose.yml")},
	}
}
