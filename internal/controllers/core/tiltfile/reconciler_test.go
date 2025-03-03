package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	configmap2 "github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/uibutton"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/configmap"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

func TestDefault(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")
	f.tempdir.WriteFile(p, "print('hello-world')")

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	ts := time.Now()
	f.Create(&tf)

	// Make sure the FileWatch was created
	var fw v1alpha1.FileWatch
	fwKey := types.NamespacedName{Name: "configs:my-tf"}
	f.MustGet(fwKey, &fw)
	assert.Equal(t, tf.Spec.Path, fw.Spec.WatchedPaths[0])

	f.waitForRunning(tf.Name)

	f.popQueue()

	f.waitForTerminatedAfter(tf.Name, ts)

	f.Delete(&tf)

	// Ensure the FileWatch was deleted.
	assert.False(t, f.Get(fwKey, &fw))
}

func TestSteadyState(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")
	f.tempdir.WriteFile(p, "print('hello-world')")

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	// Make sure a second reconcile doesn't update the status again.
	var tf2 = v1alpha1.Tiltfile{}
	f.MustReconcile(types.NamespacedName{Name: "my-tf"})
	f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf2)
	assert.Equal(t, tf.ResourceVersion, tf2.ResourceVersion)
}

func TestLiveUpdate(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	luSpec := v1alpha1.LiveUpdateSpec{
		BasePath:  f.tempdir.Path(),
		StopPaths: []string{filepath.Join("src", "package.json")},
		Syncs:     []v1alpha1.LiveUpdateSync{{LocalPath: "src", ContainerPath: "/src"}},
	}
	expectedSpec := *(luSpec.DeepCopy())
	expectedSpec.Sources = []v1alpha1.LiveUpdateSource{{
		FileWatch: "image:sancho-image",
		ImageMap:  "sancho-image",
	}}
	expectedSpec.Selector.Kubernetes = &v1alpha1.LiveUpdateKubernetesSelector{
		ImageMapName:  "sancho-image",
		DiscoveryName: "sancho",
		ApplyName:     "sancho",
	}

	sanchoImage := model.MustNewImageTarget(container.MustParseSelector("sancho-image")).
		WithLiveUpdateSpec("sancho:sancho-image", luSpec).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.tempdir.Path()})
	sancho := manifestbuilder.New(f.tempdir, "sancho").
		WithImageTargets(sanchoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{sancho},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	assert.Equal(t, "", tf.Status.Terminated.Error)

	var luList = v1alpha1.LiveUpdateList{}
	f.List(&luList)
	if assert.Equal(t, 1, len(luList.Items)) {
		assert.Equal(t, "sancho:sancho-image", luList.Items[0].Name)
		assert.Equal(t, expectedSpec, luList.Items[0].Spec)
	}
}

func TestCluster(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")
	f.r.k8sContextOverride = "context-override"
	f.r.k8sNamespaceOverride = "namespace-override"

	expected := &v1alpha1.ClusterConnection{
		Kubernetes: &v1alpha1.KubernetesClusterConnection{
			Context:   string(f.r.k8sContextOverride),
			Namespace: string(f.r.k8sNamespaceOverride),
		},
	}

	sancho := manifestbuilder.New(f.tempdir, "sancho").
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{sancho},
	}

	name := model.MainTiltfileManifestName.String()
	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	assert.Equal(t, "", tf.Status.Terminated.Error)

	var clList = v1alpha1.ClusterList{}
	f.List(&clList)
	if assert.Equal(t, 1, len(clList.Items)) {
		assert.Equal(t, "default", clList.Items[0].Name)
		assert.Equal(t, expected, clList.Items[0].Spec.Connection)
	}
}

func TestLocalServe(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	m := manifestbuilder.New(f.tempdir, "foo").WithLocalServeCmd(".").Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{m},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	assert.Equal(t, "", tf.Status.Terminated.Error)

	a := f.st.WaitForAction(t, reflect.TypeOf(ConfigsReloadedAction{})).(ConfigsReloadedAction)
	require.Equal(t, 1, len(a.Manifests))
	m = a.Manifests[0]
	require.Equal(t, model.ManifestName("foo"), m.Name)
	require.IsType(t, model.LocalTarget{}, m.DeployTarget)
	lt := m.DeployTarget.(model.LocalTarget)
	require.NotNil(t, lt.ServeCmdDisableSource, "ServeCmdDisableSource is nil")
	require.NotNil(t, lt.ServeCmdDisableSource.ConfigMap, "ServeCmdDisableSource.ConfigMap is nil")
	require.Equal(t, "foo-disable", lt.ServeCmdDisableSource.ConfigMap.Name)
}

func TestDockerMetrics(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	sanchoImage := model.MustNewImageTarget(container.MustParseSelector("sancho-image")).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.tempdir.Path()})
	sancho := manifestbuilder.New(f.tempdir, "sancho").
		WithImageTargets(sanchoImage).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()

	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{sancho},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	connectEvt := analytics.CountEvent{
		Name: "api.tiltfile.docker.connect",
		Tags: map[string]string{
			"server.arch":    "amd64",
			"server.version": "20.10.11",
			"status":         "connected",
		},
		N: 1,
	}
	assert.ElementsMatch(t, []analytics.CountEvent{connectEvt}, f.ma.Counts)
}

func TestArgsChangeResetsEnabledResources(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	m1 := manifestbuilder.New(f.tempdir, "m1").WithLocalServeCmd("hi").Build()
	m2 := manifestbuilder.New(f.tempdir, "m2").WithLocalServeCmd("hi").Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests:        []model.Manifest{m1, m2},
		EnabledManifests: []model.ManifestName{"m1", "m2"},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
			Args: []string{"m1", "m2"},
		},
	}
	f.createAndWaitForLoaded(&tf)

	ts := time.Now()

	f.setArgs("my-tf", []string{"m2"})
	f.tfl.Result.EnabledManifests = []model.ManifestName{"m2"}

	f.MustReconcile(types.NamespacedName{Name: "my-tf"})
	f.waitForRunning("my-tf")
	f.popQueue()
	f.waitForTerminatedAfter("my-tf", ts)

	f.requireEnabled(m1, false)
	f.requireEnabled(m2, true)
}

func TestRunWithoutArgsChangePreservesEnabledResources(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	m1 := manifestbuilder.New(f.tempdir, "m1").WithLocalServeCmd("hi").Build()
	m2 := manifestbuilder.New(f.tempdir, "m2").WithLocalServeCmd("hi").Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests:        []model.Manifest{m1, m2},
		EnabledManifests: []model.ManifestName{"m1", "m2"},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
			Args: []string{"m1"},
		},
	}
	f.createAndWaitForLoaded(&tf)

	err := configmap.UpsertDisableConfigMap(f.Context(), f.Client, "m2-disable", "isDisabled", false)
	require.NoError(t, err)

	f.setArgs("my-tf", tf.Spec.Args)

	f.triggerRun("my-tf")

	ts := time.Now()
	f.MustReconcile(types.NamespacedName{Name: "my-tf"})
	f.waitForRunning("my-tf")
	f.popQueue()
	f.waitForTerminatedAfter("my-tf", ts)

	f.requireEnabled(m1, true)
	f.requireEnabled(m2, true)
}

func TestTiltfileFailurePreservesEnabledResources(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	m1 := manifestbuilder.New(f.tempdir, "m1").WithLocalServeCmd("hi").Build()
	m2 := manifestbuilder.New(f.tempdir, "m2").WithLocalServeCmd("hi").Build()
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests:        []model.Manifest{m1, m2},
		EnabledManifests: []model.ManifestName{"m1"},
	}

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
			Args: []string{"m1"},
		},
	}
	f.createAndWaitForLoaded(&tf)

	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests:        []model.Manifest{m1, m2},
		EnabledManifests: []model.ManifestName{},
		Error:            errors.New("unknown manifest: m3"),
	}

	f.triggerRun("my-tf")

	ts := time.Now()
	f.MustReconcile(types.NamespacedName{Name: "my-tf"})
	f.waitForRunning("my-tf")
	f.popQueue()
	f.waitForTerminatedAfter("my-tf", ts)

	f.requireEnabled(m1, true)
	f.requireEnabled(m2, false)
}

func TestCancel(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")
	f.tempdir.WriteFile(p, "print('hello-world')")

	f.tfl.Delegate = newBlockingTiltfileLoader()

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path:   p,
			StopOn: &v1alpha1.StopOnSpec{UIButtons: []string{uibutton.StopBuildButtonName("my-tf")}},
		},
	}

	cancelButton := uibutton.StopBuildButton(tf.Name)
	err := f.Client.Create(f.Context(), cancelButton)
	require.NoError(t, err)

	ts := time.Now()
	f.Create(&tf)

	f.waitForRunning(tf.Name)

	cancelButton.Status.LastClickedAt = metav1.NowMicro()
	f.UpdateStatus(cancelButton)
	require.NoError(t, err)

	f.MustReconcile(types.NamespacedName{Name: tf.Name})

	f.popQueue()

	f.waitForTerminatedAfter(tf.Name, ts)

	f.Get(types.NamespacedName{Name: tf.Name}, &tf)
	require.NotNil(t, tf.Status.Terminated)
	require.Equal(t, "build canceled", tf.Status.Terminated.Error)
}

func TestCancelClickedBeforeLoad(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")
	f.tempdir.WriteFile(p, "print('hello-world')")

	tfl := newBlockingTiltfileLoader()
	f.tfl.Delegate = tfl

	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-tf",
		},
		Spec: v1alpha1.TiltfileSpec{
			Path:   p,
			StopOn: &v1alpha1.StopOnSpec{UIButtons: []string{uibutton.StopBuildButtonName("my-tf")}},
		},
	}

	cancelButton := uibutton.StopBuildButton(tf.Name)
	cancelButton.Status.LastClickedAt = metav1.NewMicroTime(time.Now().Add(-time.Second))
	err := f.Client.Create(f.Context(), cancelButton)
	require.NoError(t, err)

	nn := types.NamespacedName{Name: tf.Name}

	ts := time.Now()
	f.Create(&tf)

	f.waitForRunning(tf.Name)

	// give the reconciler a chance to observe the cancel button click
	f.MustReconcile(nn)

	// finish the build
	tfl.Complete()

	f.MustReconcile(nn)

	f.popQueue()

	f.waitForTerminatedAfter(tf.Name, ts)

	f.Get(nn, &tf)
	require.NotNil(t, tf.Status.Terminated)
	require.Equal(t, "", tf.Status.Terminated.Error)
}

func TestPushBaseImageIssue6486(t *testing.T) {
	f := newFixture(t)
	p := f.tempdir.JoinPath("Tiltfile")

	image1 := model.MustNewImageTarget(container.MustParseSelector("image-1")).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.tempdir.Path()})
	image2 := model.MustNewImageTarget(container.MustParseSelector("image-2")).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.tempdir.Path()})
	image3 := model.MustNewImageTarget(container.MustParseSelector("image-3")).
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.tempdir.Path()}).
		WithImageMapDeps([]string{"image-1", "image-2"})

	service1 := manifestbuilder.New(f.tempdir, "service-1").
		WithImageTargets(image1).
		WithK8sYAML(testyaml.Deployment("service-1", "image-1")).
		Build()
	service3 := manifestbuilder.New(f.tempdir, "service-3").
		WithImageTargets(image1, image2, image3).
		WithK8sYAML(testyaml.Deployment("service-3", "image-3")).
		Build()

	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{service1, service3},
	}

	name := model.MainTiltfileManifestName.String()
	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: p,
		},
	}
	f.createAndWaitForLoaded(&tf)

	assert.Equal(t, "", tf.Status.Terminated.Error)

	var imageList = v1alpha1.DockerImageList{}
	f.List(&imageList)

	sort.Slice(imageList.Items, func(i, j int) bool {
		return imageList.Items[i].Name < imageList.Items[j].Name
	})

	if assert.Equal(t, 4, len(imageList.Items)) {
		assert.Equal(t, "service-1:image-1", imageList.Items[0].Name)
		assert.Equal(t, v1alpha1.ClusterImageNeedsPush, imageList.Items[0].Spec.ClusterNeeds)
		assert.Equal(t, "service-3:image-1", imageList.Items[1].Name)
		assert.Equal(t, v1alpha1.ClusterImageNeedsPush, imageList.Items[1].Spec.ClusterNeeds)
		assert.Equal(t, "service-3:image-2", imageList.Items[2].Name)
		assert.Equal(t, v1alpha1.ClusterImageNeedsBase, imageList.Items[2].Spec.ClusterNeeds)
		assert.Equal(t, "service-3:image-3", imageList.Items[3].Name)
		assert.Equal(t, v1alpha1.ClusterImageNeedsPush, imageList.Items[3].Spec.ClusterNeeds)
	}
}

type testStore struct {
	*store.TestingStore
	out *bytes.Buffer
}

func NewTestingStore() *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          bytes.NewBuffer(nil),
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	logAction, ok := action.(store.LogAction)
	if ok {
		_, _ = fmt.Fprintf(s.out, "%s", logAction.Message())
	}
}

type fixture struct {
	*fake.ControllerFixture
	tempdir *tempdir.TempDirFixture
	st      *testStore
	r       *Reconciler
	q       workqueue.TypedRateLimitingInterface[reconcile.Request]
	tfl     *tiltfile.FakeTiltfileLoader
	ma      *analytics.MemoryAnalytics
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tf := tempdir.NewTempDirFixture(t)

	st := NewTestingStore()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	r := NewReconciler(st, tfl, d, cfb.Client, v1alpha1.NewScheme(), store.EngineModeUp, "", "", 0)
	q := workqueue.NewTypedRateLimitingQueue[reconcile.Request](
		workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](time.Millisecond, time.Millisecond))
	_ = r.requeuer.Start(context.Background(), q)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		tempdir:           tf,
		st:                st,
		r:                 r,
		q:                 q,
		tfl:               tfl,
		ma:                cfb.Analytics(),
	}
}

// Wait for the next item on the workqueue, then run reconcile on it.
func (f *fixture) popQueue() {
	f.T().Helper()

	done := make(chan error)
	go func() {
		item, _ := f.q.Get()
		_, err := f.r.Reconcile(f.Context(), item)
		f.q.Done(item)
		done <- err
	}()

	select {
	case <-time.After(time.Second):
		f.T().Fatal("timeout waiting for workqueue")
	case err := <-done:
		assert.NoError(f.T(), err)
	}
}

func (f *fixture) waitForTerminatedAfter(name string, ts time.Time) {
	require.Eventually(f.T(), func() bool {
		var tf v1alpha1.Tiltfile
		f.MustGet(types.NamespacedName{Name: name}, &tf)
		return tf.Status.Terminated != nil && tf.Status.Terminated.FinishedAt.After(ts)
	}, time.Second, time.Millisecond, "waiting for tiltfile to finish running")
}

func (f *fixture) waitForRunning(name string) {
	require.Eventually(f.T(), func() bool {
		var tf v1alpha1.Tiltfile
		f.MustGet(types.NamespacedName{Name: name}, &tf)
		return tf.Status.Running != nil
	}, time.Second, time.Millisecond, "waiting for tiltfile to start running")
}

func (f *fixture) createAndWaitForLoaded(tf *v1alpha1.Tiltfile) {
	ts := time.Now()
	f.Create(tf)

	f.waitForRunning(tf.Name)

	f.popQueue()

	f.waitForTerminatedAfter(tf.Name, ts)

	f.MustGet(types.NamespacedName{Name: tf.Name}, tf)
}

func (f *fixture) triggerRun(name string) {
	queue := configmap2.TriggerQueueCreate([]configmap2.TriggerQueueEntry{{Name: model.ManifestName(name)}})
	f.Create(&queue)
}

func (f *fixture) setArgs(name string, args []string) {
	tf := v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	_, err := controllerutil.CreateOrUpdate(f.Context(), f.Client, &tf, func() error {
		tf.Spec.Args = args
		return nil
	})
	require.NoError(f.T(), err)
}

func (f *fixture) requireEnabled(m model.Manifest, isEnabled bool) {
	var cm v1alpha1.ConfigMap
	f.MustGet(types.NamespacedName{Name: disableConfigMapName(m)}, &cm)
	isDisabled, err := strconv.ParseBool(cm.Data["isDisabled"])
	require.NoError(f.T(), err)
	actualIsEnabled := !isDisabled
	require.Equal(f.T(), isEnabled, actualIsEnabled, "is %s enabled", m.Name)
}

// builds block until canceled or manually completed
type blockingTiltfileLoader struct {
	completionChan chan struct{}
}

func newBlockingTiltfileLoader() blockingTiltfileLoader {
	return blockingTiltfileLoader{completionChan: make(chan struct{})}
}

func (b blockingTiltfileLoader) Load(ctx context.Context, tf *v1alpha1.Tiltfile, prevResult *tiltfile.TiltfileLoadResult) tiltfile.TiltfileLoadResult {
	select {
	case <-ctx.Done():
	case <-b.completionChan:
	}
	return tiltfile.TiltfileLoadResult{}
}

func (b blockingTiltfileLoader) Complete() {
	close(b.completionChan)
}
