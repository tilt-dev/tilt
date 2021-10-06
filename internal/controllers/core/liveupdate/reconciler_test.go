package liveupdate

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
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

type fixture struct {
	*fake.ControllerFixture
	r *Reconciler
}

func newFixture(t testing.TB) *fixture {
	cfb := fake.NewControllerFixtureBuilder(t)
	cu := &containerupdate.FakeContainerUpdater{}
	r := NewFakeReconciler(cu, cfb.Client)
	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
	}
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
