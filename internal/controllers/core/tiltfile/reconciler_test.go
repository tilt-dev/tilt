package tiltfile

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
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
	f.Create(&tf)

	// Make sure the FileWatch was created
	var fw v1alpha1.FileWatch
	fwKey := types.NamespacedName{Name: "configs:my-tf"}
	f.MustGet(fwKey, &fw)
	assert.Equal(t, tf.Spec.Path, fw.Spec.WatchedPaths[0])

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Running != nil
	}, time.Second, time.Millisecond)

	f.popQueue()

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Terminated != nil
	}, time.Second, time.Millisecond)

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
	f.Create(&tf)

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Running != nil
	}, time.Second, time.Millisecond)

	f.popQueue()

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Terminated != nil
	}, time.Second, time.Millisecond)

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
	expectedSpec.FileWatchNames = []string{"image:sancho-image"}
	expectedSpec.Selector.Kubernetes = &v1alpha1.LiveUpdateKubernetesSelector{
		Image:         "sancho-image",
		DiscoveryName: "sancho",
		ApplyName:     "sancho",
		ImageMapName:  "sancho-image",
	}

	sanchoImage := model.MustNewImageTarget(container.MustParseSelector("sancho-image")).
		WithLiveUpdateSpec("sancho:sancho-image", luSpec).
		WithBuildDetails(model.DockerBuild{BuildPath: f.tempdir.Path()})
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
	f.Create(&tf)

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Running != nil
	}, time.Second, time.Millisecond)

	f.popQueue()

	assert.Eventually(t, func() bool {
		f.MustGet(types.NamespacedName{Name: "my-tf"}, &tf)
		return tf.Status.Terminated != nil
	}, time.Second, time.Millisecond)

	assert.Equal(t, "", tf.Status.Terminated.Error)

	var luList = v1alpha1.LiveUpdateList{}
	f.List(&luList)
	if assert.Equal(t, 1, len(luList.Items)) {
		assert.Equal(t, "sancho:sancho-image", luList.Items[0].Name)
		assert.Equal(t, expectedSpec, luList.Items[0].Spec)
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
	bs      *BuildSource
	q       workqueue.RateLimitingInterface
	tfl     *tiltfile.FakeTiltfileLoader
}

func newFixture(t *testing.T) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	tf := tempdir.NewTempDirFixture(t)
	t.Cleanup(tf.TearDown)

	st := NewTestingStore()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	bs := NewBuildSource()
	r := NewReconciler(st, tfl, d, cfb.Client, v1alpha1.NewScheme(), bs, store.EngineModeUp)
	q := workqueue.NewRateLimitingQueue(
		workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, time.Millisecond))
	_ = bs.Start(context.Background(), handler.Funcs{}, q)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		tempdir:           tf,
		st:                st,
		r:                 r,
		bs:                bs,
		q:                 q,
		tfl:               tfl,
	}
}

// Wait for the next item on the workqueue, then run reconcile on it.
func (f *fixture) popQueue() {
	f.T().Helper()

	done := make(chan error)
	go func() {
		item, _ := f.q.Get()
		_, err := f.r.Reconcile(f.Context(), item.(reconcile.Request))
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
