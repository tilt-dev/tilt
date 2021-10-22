package liveupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestIndexing(t *testing.T) {
	f := newFixture(t)

	// KubernetesDiscovery + KubernetesApply
	f.Create(&v1alpha1.LiveUpdate{
		ObjectMeta: metav1.ObjectMeta{Name: "kdisco-kapply"},
		Spec: v1alpha1.LiveUpdateSpec{
			BasePath: "/tmp",
			Selector: kubernetesSelector("discovery", "apply", "fake-image"),
			Syncs: []v1alpha1.LiveUpdateSync{
				{LocalPath: "in", ContainerPath: "/out/"},
			},
		},
	})

	// KubernetesDiscovery w/o Kubernetes Apply
	f.Create(&v1alpha1.LiveUpdate{
		ObjectMeta: metav1.ObjectMeta{Name: "no-kapply"},
		Spec: v1alpha1.LiveUpdateSpec{
			BasePath: "/tmp",
			Selector: kubernetesSelector("discovery", "", "fake-image"),
			Syncs: []v1alpha1.LiveUpdateSync{
				{LocalPath: "in", ContainerPath: "/out/"},
			},
		},
	})

	reqs := f.r.indexer.Enqueue(&v1alpha1.KubernetesDiscovery{ObjectMeta: metav1.ObjectMeta{Name: "discovery"}})
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "kdisco-kapply"}},
		{NamespacedName: types.NamespacedName{Name: "no-kapply"}},
	}, reqs)

	reqs = f.r.indexer.Enqueue(&v1alpha1.KubernetesApply{ObjectMeta: metav1.ObjectMeta{Name: "apply"}})
	require.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: "kdisco-kapply"}},
	}, reqs)
}

func TestConsumeFileEvents(t *testing.T) {
	f := newFixture(t)

	p, _ := os.Getwd()
	nowMicro := apis.NowMicro()
	txtPath := filepath.Join(p, "a.txt")
	txtChangeTime := metav1.MicroTime{Time: nowMicro.Add(time.Second)}

	f.setupFrontend()

	// Verify initial setup.
	m, ok := f.r.monitors["frontend-liveupdate"]
	require.True(t, ok)
	assert.Equal(t, map[string]metav1.MicroTime{}, m.modTimeByPath)
	assert.Equal(t, "frontend-discovery", m.lastKubernetesDiscovery.Name)

	// Trigger a file event, and make sure it gets pulled into the monitor.
	f.addFileEvent("frontend-fw", txtPath, txtChangeTime)
	f.MustReconcile(types.NamespacedName{Name: "frontend-liveupdate"})
	assert.Equal(t, map[string]metav1.MicroTime{
		txtPath: txtChangeTime,
	}, m.modTimeByPath)
}

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t testing.TB) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	cu := &containerupdate.FakeContainerUpdater{}
	st := store.NewTestingStore()
	r := NewFakeReconciler(st, cu, cfb.Client)
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
}

func (f *fixture) addFileEvent(name string, p string, time metav1.MicroTime) {
	var fw v1alpha1.FileWatch
	f.MustGet(types.NamespacedName{Name: name}, &fw)
	update := fw.DeepCopy()
	update.Status.FileEvents = append(update.Status.FileEvents, v1alpha1.FileEvent{
		Time:      time,
		SeenFiles: []string{p},
	})
	f.UpdateStatus(update)
}

// Create a frontend LiveUpdate with all objects attached.
func (f *fixture) setupFrontend() {
	p, _ := os.Getwd()
	now := apis.Now()
	nowMicro := apis.NowMicro()

	// Create all the objects.
	f.Create(&v1alpha1.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-fw"},
		Spec: v1alpha1.FileWatchSpec{
			WatchedPaths: []string{p},
		},
		Status: v1alpha1.FileWatchStatus{
			MonitorStartTime: nowMicro,
		},
	})
	f.Create(&v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-apply"},
		Status:     v1alpha1.KubernetesApplyStatus{},
	})
	f.Create(&v1alpha1.ImageMap{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-image-map"},
		Status: v1alpha1.ImageMapStatus{
			Image:          "frontend-image:my-tag",
			BuildStartTime: &nowMicro,
		},
	})
	f.Create(&v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-discovery"},
		Status: v1alpha1.KubernetesDiscoveryStatus{
			MonitorStartTime: nowMicro,
			Pods: []v1alpha1.Pod{
				{
					Name:      "pod-1",
					Namespace: "default",
					Containers: []v1alpha1.Container{
						{
							Name:  "main",
							ID:    "main",
							Image: "frontend-image",
							Ready: true,
							State: v1alpha1.ContainerState{
								Running: &v1alpha1.ContainerStateRunning{
									StartedAt: now,
								},
							},
						},
					},
				},
			},
		},
	})
	f.Create(&v1alpha1.LiveUpdate{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend-liveupdate"},
		Spec: v1alpha1.LiveUpdateSpec{
			BasePath:       p,
			FileWatchNames: []string{"frontend-fw"},
			Selector: v1alpha1.LiveUpdateSelector{
				Kubernetes: &v1alpha1.LiveUpdateKubernetesSelector{
					ApplyName:     "frontend-apply",
					DiscoveryName: "frontend-discovery",
					Image:         "frontend-image",
					ImageMapName:  "frontend-image-map",
				},
			},
			Syncs: []v1alpha1.LiveUpdateSync{
				{LocalPath: ".", ContainerPath: "/app"},
			},
		},
	})
}

func kubernetesSelector(discoveryName string, applyName string, image string) v1alpha1.LiveUpdateSelector {
	return v1alpha1.LiveUpdateSelector{
		Kubernetes: &v1alpha1.LiveUpdateKubernetesSelector{
			DiscoveryName: discoveryName,
			ApplyName:     applyName,
			Image:         image,
		},
	}
}
