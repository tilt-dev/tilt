package engine

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ktypes "k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var testImageRef = container.MustParseNamedTagged("gcr.io/some-project-162817/sancho:deadbeef")
var imageTargetID = model.TargetID{
	Type: model.TargetTypeImage,
	Name: model.TargetName(apis.SanitizeName("gcr.io/some-project-162817/sancho")),
}

var alreadyBuilt = store.NewImageBuildResultSingleRef(imageTargetID, testImageRef)
var alreadyBuiltSet = store.BuildResultSet{imageTargetID: alreadyBuilt}

type expectedFile = testutils.ExpectedFile

func TestGKEDeploy(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	manifest := NewSanchoLiveUpdateManifest(f)
	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}

	expectedYaml := "image: gcr.io/some-project-162817/sancho:tilt-11cd0b38bc3ceb95"
	if !strings.Contains(f.k8s.Yaml, expectedYaml) {
		t.Errorf("Expected yaml to contain %q. Actual:\n%s", expectedYaml, f.k8s.Yaml)
	}
}

func TestYamlManifestDeploy(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	manifest := manifestbuilder.New(f, "some_yaml").
		WithK8sYAML(testyaml.TracerYAML).Build()
	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 0, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	f.assertK8sUpsertCalled(true)
}

func TestFallBackToImageDeploy(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductDockerDesktop, container.RuntimeDocker)

	f.docker.SetExecError(errors.New("some random error"))

	manifest := NewSanchoLiveUpdateManifest(f)
	changed := f.WriteFile("a.txt", "a")
	bs := resultToStateSet(manifest, alreadyBuiltSet, []string{changed})

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, bs)
	if err != nil {
		t.Fatal(err)
	}

	f.assertContainerRestarts(0)
	if f.docker.BuildCount != 1 {
		t.Errorf("Expected 1 docker build, actual: %d", f.docker.BuildCount)
	}
}

func TestIgnoredFiles(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductDockerDesktop, container.RuntimeDocker)

	manifest := NewSanchoDockerBuildManifest(f)

	tiltfile := filepath.Join(f.Path(), "Tiltfile")
	manifest = manifest.WithImageTarget(manifest.ImageTargetAt(0).WithIgnores([]v1alpha1.IgnoreDef{
		{BasePath: filepath.Join(f.Path(), ".git")},
		{BasePath: tiltfile},
	}))

	f.WriteFile("Tiltfile", "# hello world")
	f.WriteFile("a.txt", "a")
	f.WriteFile(".git/index", "garbage")

	targets := buildcontrol.BuildTargets(manifest)
	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	tr := tar.NewReader(f.docker.BuildContext)
	testutils.AssertFilesInTar(t, tr, []expectedFile{
		expectedFile{
			Path:     "a.txt",
			Contents: "a",
		},
		expectedFile{
			Path:    ".git/index",
			Missing: true,
		},
		expectedFile{
			Path:    "Tiltfile",
			Missing: true,
		},
	})
}

func TestCustomBuild(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images["gcr.io/some-project-162817/sancho:tilt-build-1551202573"] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifest(f)
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestCustomBuildDeterministicTag(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)
	refStr := "gcr.io/some-project-162817/sancho:deterministic-tag"
	sha := digest.Digest("sha256:11cd0eb38bc3ceb958ffb2f9bd70be3fb317ce7d255c8a4c3f4af30e298aa1aab")
	f.docker.Images[refStr] = types.ImageInspect{ID: string(sha)}

	manifest := NewSanchoCustomBuildManifestWithTag(f, "deterministic-tag")
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	if f.docker.BuildCount != 0 {
		t.Errorf("Expected 0 docker build, actual: %d", f.docker.BuildCount)
	}

	if f.docker.PushCount != 1 {
		t.Errorf("Expected 1 push to docker, actual: %d", f.docker.PushCount)
	}
}

func TestDockerComposeImageBuild(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	manifest := NewSanchoLiveUpdateDCManifest(f)
	targets := buildcontrol.BuildTargets(manifest)

	_, err := f.BuildAndDeploy(targets, store.BuildStateSet{})
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.docker.BuildCount)
	assert.Equal(t, 0, f.docker.PushCount)
	assert.Empty(t, f.k8s.Yaml, "expect no k8s YAML for DockerCompose resource")
	assert.Len(t, f.dcCli.UpCalls(), 1)
}

func TestReturnLastUnexpectedError(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductDockerDesktop, container.RuntimeDocker)

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoLiveUpdateManifest(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}
}

// errors get logged by the upper, so make sure our builder isn't logging the error redundantly
func TestDockerBuildErrorNotLogged(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	// next Docker build will throw an unexpected error -- this is one we want to return,
	// even if subsequent builders throw expected errors.
	f.docker.BuildErrorToThrow = fmt.Errorf("no one expects the unexpected error")

	manifest := NewSanchoDockerBuildManifest(f)
	_, err := f.BuildAndDeploy(buildcontrol.BuildTargets(manifest), store.BuildStateSet{})
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no one expects the unexpected error")
	}

	logs := f.logs.String()
	require.Equal(t, 0, strings.Count(logs, "no one expects the unexpected error"))
}

func TestLocalTargetDeploy(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	lt := model.NewLocalTarget("hello-world", model.ToHostCmd("echo hello world"), model.Cmd{}, nil)
	res, err := f.BuildAndDeploy([]model.TargetSpec{lt}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, 0, f.docker.BuildCount, "should have 0 docker builds")
	assert.Equal(t, 0, f.docker.PushCount, "should have 0 docker pushes")
	assert.Empty(t, f.k8s.Yaml, "should not apply any k8s yaml")
	assert.Len(t, res, 1, "expect exactly one result in result set")
	assert.Contains(t, f.logs.String(), "hello world", "logs should contain cmd output")
}

func TestLocalTargetFailure(t *testing.T) {
	f := newBDFixture(t, clusterid.ProductGKE, container.RuntimeDocker)

	lt := model.NewLocalTarget("hello-world", model.ToHostCmd("echo 'oh no' && exit 1"), model.Cmd{}, nil)
	res, err := f.BuildAndDeploy([]model.TargetSpec{lt}, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty result for failed command")

	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "exit status 1", "error msg should indicate command failure")
	assert.Contains(t, f.logs.String(), "oh no", "logs should contain cmd output")

	assert.Equal(t, 0, f.docker.BuildCount, "should have 0 docker builds")
	assert.Equal(t, 0, f.docker.PushCount, "should have 0 docker pushes")
	assert.Empty(t, f.k8s.Yaml, "should not apply any k8s yaml")
}

type testStore struct {
	*store.TestingStore
	out io.Writer
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	if action, ok := action.(store.LogAction); ok {
		_, _ = s.out.Write(action.Message())
	}
}

// The API boundaries between BuildAndDeployer and the ImageBuilder aren't obvious and
// are likely to change in the future. So we test them together, using
// a fake Client and K8sClient
type bdFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	cancel     func()
	docker     *docker.FakeClient
	k8s        *k8s.FakeK8sClient
	bd         buildcontrol.BuildAndDeployer
	st         *testStore
	dcCli      *dockercompose.FakeDCClient
	logs       *bytes.Buffer
	ctrlClient ctrlclient.Client
}

func newBDFixture(t *testing.T, env clusterid.Product, runtime container.Runtime) *bdFixture {
	return newBDFixtureWithUpdateMode(t, env, runtime, liveupdates.UpdateModeAuto)
}

func newBDFixtureWithUpdateMode(t *testing.T, env clusterid.Product, runtime container.Runtime, um liveupdates.UpdateMode) *bdFixture {
	logs := new(bytes.Buffer)
	ctx, _, ta := testutils.ForkedCtxAndAnalyticsForTest(logs)
	ctx, cancel := context.WithCancel(ctx)
	f := tempdir.NewTempDirFixture(t)
	dir := dirs.NewTiltDevDirAt(f.Path())
	dockerClient := docker.NewFakeClient()
	dockerClient.ContainerListOutput = map[string][]types.Container{
		"pod": []types.Container{
			types.Container{
				ID: k8s.MagicTestContainerID,
			},
		},
	}
	k8s := k8s.NewFakeK8sClient(t)
	k8s.Runtime = runtime
	mode := liveupdates.UpdateModeFlag(um)
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kl := &fakeKINDLoader{}
	ctrlClient := fake.NewFakeTiltClient()
	st := NewTestingStore(logs)
	execer := localexec.NewFakeExecer(t)
	bd, err := provideFakeBuildAndDeployer(ctx, dockerClient, k8s, dir, env, mode, dcc,
		fakeClock{now: time.Unix(1551202573, 0)}, kl, ta, ctrlClient, st, execer)
	require.NoError(t, err)

	ret := &bdFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		docker:         dockerClient,
		k8s:            k8s,
		bd:             bd,
		st:             st,
		dcCli:          dcc,
		logs:           logs,
		ctrlClient:     ctrlClient,
	}

	t.Cleanup(ret.TearDown)
	return ret
}

func (f *bdFixture) TearDown() {
	f.cancel()
}

func (f *bdFixture) NewPathSet(paths ...string) model.PathSet {
	return model.NewPathSet(paths, f.Path())
}

func (f *bdFixture) assertContainerRestarts(count int) {
	// Ensure that MagicTestContainerID was the only container id that saw
	// restarts, and that it saw the right number of restarts.
	expected := map[string]int{}
	if count != 0 {
		expected[string(k8s.MagicTestContainerID)] = count
	}
	assert.Equal(f.T(), expected, f.docker.RestartsByContainer,
		"checking for expected # of container restarts")
}

func (f *bdFixture) assertK8sUpsertCalled(called bool) {
	assert.Equal(f.T(), called, f.k8s.Yaml != "",
		"checking that k8s.Upsert was called")
}

func (f *bdFixture) upsert(obj ctrlclient.Object) {
	require.True(f.T(), obj.GetName() != "",
		"object has empty name")

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

func (f *bdFixture) BuildAndDeploy(specs []model.TargetSpec, stateSet store.BuildStateSet) (store.BuildResultSet, error) {
	cluster := &v1alpha1.Cluster{}
	for _, spec := range specs {
		switch spec.(type) {
		case model.DockerComposeTarget:
			cluster.Spec.Connection = &v1alpha1.ClusterConnection{
				Docker: &v1alpha1.DockerClusterConnection{},
			}
		case model.K8sTarget:
			cluster.Spec.Connection = &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			}
		}
	}

	for _, spec := range specs {
		localTarget, ok := spec.(model.LocalTarget)
		if ok && localTarget.UpdateCmdSpec != nil {
			cmd := v1alpha1.Cmd{
				ObjectMeta: metav1.ObjectMeta{Name: localTarget.UpdateCmdName()},
				Spec:       *(localTarget.UpdateCmdSpec),
			}
			f.upsert(&cmd)
		}

		iTarget, ok := spec.(model.ImageTarget)
		if ok {
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

			if !liveupdate.IsEmptySpec(iTarget.LiveUpdateSpec) {
				lu := v1alpha1.LiveUpdate{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.LiveUpdateName},
					Spec:       iTarget.LiveUpdateSpec,
				}
				f.upsert(&lu)
			}

			if iTarget.IsDockerBuild() {
				di := v1alpha1.DockerImage{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.DockerImageName},
					Spec:       iTarget.DockerBuildInfo().DockerImageSpec,
				}
				f.upsert(&di)
			}
			if iTarget.IsCustomBuild() {
				cmdImageSpec := iTarget.CustomBuildInfo().CmdImageSpec
				ci := v1alpha1.CmdImage{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.CmdImageName},
					Spec:       cmdImageSpec,
				}
				f.upsert(&ci)

				c := v1alpha1.Cmd{
					ObjectMeta: metav1.ObjectMeta{Name: iTarget.CmdImageName},
					Spec: v1alpha1.CmdSpec{
						Args: cmdImageSpec.Args,
						Dir:  cmdImageSpec.Dir,
					},
				}
				f.upsert(&c)
			}
		}

		kTarget, ok := spec.(model.K8sTarget)
		if ok {
			ka := v1alpha1.KubernetesApply{
				ObjectMeta: metav1.ObjectMeta{Name: kTarget.ID().Name.String()},
				Spec:       kTarget.KubernetesApplySpec,
			}
			f.upsert(&ka)
		}

		dcTarget, ok := spec.(model.DockerComposeTarget)
		if ok {
			dcs := v1alpha1.DockerComposeService{
				ObjectMeta: metav1.ObjectMeta{Name: dcTarget.ID().Name.String()},
				Spec:       dcTarget.Spec,
			}
			f.upsert(&dcs)
		}
	}
	return f.bd.BuildAndDeploy(f.ctx, f.st, specs, stateSet)
}

func resultToStateSet(m model.Manifest, resultSet store.BuildResultSet, files []string) store.BuildStateSet {
	stateSet := store.BuildStateSet{}
	for id, result := range resultSet {
		stateSet[id] = store.NewBuildState(result, files, nil)
	}
	return stateSet
}

type fakeClock struct {
	now time.Time
}

func (c fakeClock) Now() time.Time { return c.now }

type fakeKINDLoader struct {
	loadCount int
}

func (kl *fakeKINDLoader) LoadToKIND(ctx context.Context, cluster *v1alpha1.Cluster, ref reference.NamedTagged) error {
	kl.loadCount++
	return nil
}
