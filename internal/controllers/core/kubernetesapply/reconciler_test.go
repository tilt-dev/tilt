package kubernetesapply

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestImageIndexing(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ImageMaps: []string{"image-a", "image-c"},
		},
	}
	f.Create(&ka)

	// Verify we can index one image map.
	reqs := f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-a"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "a"}},
	}, reqs)

	kb := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "b",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			ImageMaps: []string{"image-b", "image-c"},
		},
	}
	f.Create(&kb)

	// Verify we can index one image map to two applies.
	reqs = f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "a"}},
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "b"}},
	}, reqs)

	ka.Spec.ImageMaps = []string{"image-a"}
	f.Update(&ka)

	// Verify we can remove an image map.
	reqs = f.r.indexer.Enqueue(&v1alpha1.ImageMap{ObjectMeta: metav1.ObjectMeta{Name: "image-c"}})
	assert.ElementsMatch(t, []reconcile.Request{
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "b"}},
	}, reqs)
}

func TestBasicApply(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Contains(f.T(), ka.Status.ResultYAML, "name: sancho")
	assert.Contains(f.T(), ka.Status.ResultYAML, "uid:")

	// Make sure that re-reconciling doesn't re-apply the YAML"
	f.kClient.Yaml = ""
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Equal(f.T(), f.kClient.Yaml, "")
}

func TestGarbageCollectAll(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: testyaml.SanchoYAML,
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")

	f.Delete(&ka)
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.DeletedYaml, "name: sancho")
}

func TestGarbageCollectPartial(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML: fmt.Sprintf("%s\n---\n%s\n", testyaml.SanchoYAML, testyaml.PodDisruptionBudgetYAML),
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.Contains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")

	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	ka.Spec.YAML = testyaml.SanchoYAML
	f.Update(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.Contains(f.T(), f.kClient.Yaml, "name: sancho")
	assert.NotContains(f.T(), f.kClient.Yaml, "name: infra-kafka-zookeeper")
	assert.Contains(f.T(), f.kClient.DeletedYaml, "name: infra-kafka-zookeeper")
}

type fixture struct {
	*fake.ControllerFixture
	r       *Reconciler
	kClient *k8s.FakeK8sClient
}

func newFixture(t *testing.T) *fixture {
	kClient := k8s.NewFakeK8sClient(t)
	cfb := fake.NewControllerFixtureBuilder(t)
	st := store.NewTestingStore()
	dockerClient := docker.NewFakeClient()
	kubeContext := k8s.KubeContext("kind-kind")

	// Make the fake ImageExists always return true, which is the behavior we want
	// when testing the reconciler
	dockerClient.ImageAlwaysExists = true

	db := build.NewDockerImageBuilder(dockerClient, dockerfile.Labels{})
	r := NewReconciler(cfb.Client, kClient, v1alpha1.NewScheme(), db, kubeContext, st)

	return &fixture{
		ControllerFixture: cfb.Build(r),
		r:                 r,
		kClient:           kClient,
	}
}
