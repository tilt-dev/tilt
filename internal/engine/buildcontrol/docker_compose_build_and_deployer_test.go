package buildcontrol

import (
	"archive/tar"
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDockerComposeTargetBuilt(t *testing.T) {
	f := newDCBDFixture(t)

	expectedContainerID := "fake-container-id"
	f.dcCli.ContainerIDDefault = container.ID(expectedContainerID)

	manifest := manifestbuilder.New(f, "fe").WithDockerCompose().Build()
	dcTarg := manifest.DockerComposeTarget()

	res, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}
	upCalls := f.dcCli.UpCalls()
	if assert.Len(t, upCalls, 1, "expect one call to `docker-compose up`") {
		call := upCalls[0]
		assert.Equal(t, dcTarg.Spec, call.Spec)
		assert.Equal(t, "fe", call.Spec.Service)
		assert.True(t, call.ShouldBuild)
	}

	dRes := res[dcTarg.ID()].(store.DockerComposeBuildResult)
	assert.Equal(t, expectedContainerID, dRes.Status.ContainerID)
}

func TestTiltBuildsImage(t *testing.T) {
	f := newDCBDFixture(t)

	iTarget := NewSanchoDockerBuildImageTarget(f)
	manifest := manifestbuilder.New(f, "fe").
		WithDockerCompose().
		WithImageTarget(iTarget).
		Build()
	dcTarg := manifest.DockerComposeTarget()

	res, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount, "expect one docker build")

	expectedTag := fmt.Sprintf("%s:%s", iTarget.ImageMapSpec.Selector, docker.TagLatest)
	assert.Equal(t, expectedTag, f.dCli.TagTarget)

	upCalls := f.dcCli.UpCalls()
	if assert.Len(t, upCalls, 1, "expect one call to `docker-compose up`") {
		call := upCalls[0]
		assert.Equal(t, dcTarg.Spec, call.Spec)
		assert.Equal(t, "fe", call.Spec.Service)
		assert.False(t, call.ShouldBuild, "should call `up` without `--build` b/c Tilt is doing the building")
	}

	assert.Len(t, res, 2, "expect two results (one for each spec)")
}

func TestTiltBuildsImageWithTag(t *testing.T) {
	f := newDCBDFixture(t)

	refWithTag := "gcr.io/foo:bar"
	iTarget := model.MustNewImageTarget(container.MustParseSelector(refWithTag)).
		WithBuildDetails(model.DockerBuild{DockerImageSpec: v1alpha1.DockerImageSpec{Context: "-"}})
	manifest := manifestbuilder.New(f, "fe").
		WithDockerCompose().
		WithImageTarget(iTarget).
		Build()

	_, err := f.BuildAndDeploy(BuildTargets(manifest), store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, refWithTag, f.dCli.TagTarget)
}

func TestDCBADRejectsAllSpecsIfOneUnsupported(t *testing.T) {
	f := newDCBDFixture(t)

	specs := []model.TargetSpec{model.DockerComposeTarget{}, model.ImageTarget{}, model.K8sTarget{}}

	plan, err := f.dcbad.extract(specs)
	assert.Empty(t, plan)
	assert.EqualError(t, err, "DockerComposeBuildAndDeployer does not support target type model.K8sTarget")
}

func TestMultiStageDockerCompose(t *testing.T) {
	f := newDCBDFixture(t)

	manifest := NewSanchoDockerBuildMultiStageManifest(f).
		WithDeployTarget(defaultDockerComposeTarget(f, "sancho"))

	stateSet := store.BuildStateSet{}
	_, err := f.BuildAndDeploy(BuildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, f.dCli.BuildCount)
	assert.Equal(t, 0, f.dCli.PushCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:latest
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.dCli.BuildContext), expected)
}

func TestMultiStageDockerComposeWithOnlyOneDirtyImage(t *testing.T) {
	f := newDCBDFixture(t)

	manifest := NewSanchoDockerBuildMultiStageManifest(f).
		WithDeployTarget(defaultDockerComposeTarget(f, "sancho"))

	iTargetID := manifest.ImageTargets[0].ID()
	result := store.NewImageBuildResultSingleRef(iTargetID, container.MustParseNamedTagged("sancho-base:tilt-prebuilt"))
	state := store.NewBuildState(result, nil, nil)
	stateSet := store.BuildStateSet{iTargetID: state}
	_, err := f.BuildAndDeploy(BuildTargets(manifest), stateSet)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.dCli.BuildCount)
	assert.Equal(t, 0, f.dCli.PushCount)

	expected := testutils.ExpectedFile{
		Path: "Dockerfile",
		Contents: `
FROM sancho-base:tilt-prebuilt
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`,
	}
	testutils.AssertFileInTar(t, tar.NewReader(f.dCli.BuildContext), expected)
}

func TestForceUpdateDC(t *testing.T) {
	f := newDCBDFixture(t)

	m := manifestbuilder.New(f, "fe").WithDockerCompose().Build()
	iTargetID1 := m.ImageTargets[0].ID()
	stateSet := store.BuildStateSet{
		iTargetID1: store.BuildState{FullBuildTriggered: true},
	}

	_, err := f.BuildAndDeploy(BuildTargets(m), stateSet)
	require.NoError(t, err)

	// A force rebuild should delete the old resources.
	assert.Equal(t, 1, len(f.dcCli.RmCalls()))
}

type dcbdFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	dcCli      *dockercompose.FakeDCClient
	dCli       *docker.FakeClient
	dcbad      *DockerComposeBuildAndDeployer
	st         *store.TestingStore
	ctrlClient ctrlclient.Client
}

func newDCBDFixture(t *testing.T) *dcbdFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	f := tempdir.NewTempDirFixture(t)

	// empty dirs for build contexts
	_ = os.Mkdir(f.JoinPath("sancho"), 0777)
	_ = os.Mkdir(f.JoinPath("sancho-base"), 0777)

	dir := dirs.NewTiltDevDirAt(f.Path())
	dcCli := dockercompose.NewFakeDockerComposeClient(t, ctx)
	dCli := docker.NewFakeClient()
	cdc := fake.NewFakeTiltClient()
	st := store.NewTestingStore()

	// Make the fake ImageExists always return true, which is the behavior we want
	// when testing the BuildAndDeployers.
	dCli.ImageAlwaysExists = true

	clock := clockwork.NewFakeClock()
	dcbad, err := ProvideDockerComposeBuildAndDeployer(ctx, dcCli, dCli, cdc, st, clock, dir)
	if err != nil {
		t.Fatal(err)
	}
	return &dcbdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		dcCli:          dcCli,
		dCli:           dCli,
		dcbad:          dcbad,
		st:             st,
		ctrlClient:     cdc,
	}
}

func (f *dcbdFixture) upsert(obj ctrlclient.Object) {
	err := f.ctrlClient.Create(f.ctx, obj)
	if err == nil {
		return
	}

	copy := obj.DeepCopyObject().(ctrlclient.Object)
	err = f.ctrlClient.Get(f.ctx, ktypes.NamespacedName{Name: obj.GetName()}, copy)
	assert.NoError(f.T(), err)

	obj.SetResourceVersion(copy.GetResourceVersion())

	err = f.ctrlClient.Update(f.ctx, obj)
	assert.NoError(f.T(), err)
}

func (f *dcbdFixture) BuildAndDeploy(specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {

	cluster := &v1alpha1.Cluster{
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Docker: &v1alpha1.DockerClusterConnection{},
			},
		},
	}

	for _, spec := range specs {
		iTarget, ok := spec.(model.ImageTarget)
		if !ok || iTarget.IsLiveUpdateOnly {
			continue
		}

		im := v1alpha1.ImageMap{
			ObjectMeta: metav1.ObjectMeta{Name: iTarget.ID().Name.String()},
			Spec:       iTarget.ImageMapSpec,
		}
		state := stateSet[iTarget.ID()]
		state.Cluster = cluster
		stateSet[iTarget.ID()] = state

		imageBuildResult, ok := state.LastResult.(store.ImageBuildResult)
		if ok {
			im.Status = imageBuildResult.ImageMapStatus
		}
		f.upsert(&im)
	}
	return f.dcbad.BuildAndDeploy(f.ctx, f.st, specs, stateSet)
}

func defaultDockerComposeTarget(f Fixture, name string) model.DockerComposeTarget {
	return model.DockerComposeTarget{
		Name: model.TargetName(name),
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: name,
		},
	}
}
