package kubernetesapply

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestCreateAndUpdateDisco(t *testing.T) {
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
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	var kd v1alpha1.KubernetesDiscovery
	f.MustGet(types.NamespacedName{Name: "a"}, &kd)
	assert.Equal(f.T(), 1, len(kd.Spec.Watches))

	// Make sure the UID in the watch ref matches what we deployed.
	uid1 := kd.Spec.Watches[0].UID
	assert.Contains(f.T(), ka.Status.ResultYAML, fmt.Sprintf("uid: %s", uid1))

	// Change the yaml and redeploy.
	ka.Spec = v1alpha1.KubernetesApplySpec{
		YAML: strings.Replace(testyaml.SanchoYAML, "name: sancho", "name: sancho2", 1),
	}
	f.Update(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	f.MustGet(types.NamespacedName{Name: "a"}, &kd)

	// Make sure the UID changed and got updated.
	assert.Equal(f.T(), 1, len(kd.Spec.Watches))
	uid2 := kd.Spec.Watches[0].UID
	assert.NotEqual(f.T(), uid1, uid2)
	assert.Contains(f.T(), ka.Status.ResultYAML, fmt.Sprintf("uid: %s", uid2))
}

func TestDiscoveryStrategySelectorsOnly(t *testing.T) {
	f := newFixture(t)
	ka := v1alpha1.KubernetesApply{
		ObjectMeta: metav1.ObjectMeta{
			Name: "a",
		},
		Spec: v1alpha1.KubernetesApplySpec{
			YAML:              testyaml.SanchoYAML,
			DiscoveryStrategy: v1alpha1.KubernetesDiscoveryStrategySelectorsOnly,
			KubernetesDiscoveryTemplateSpec: &v1alpha1.KubernetesDiscoveryTemplateSpec{
				ExtraSelectors: []metav1.LabelSelector{
					metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "tilt-site"},
					},
				},
			},
		},
	}
	f.Create(&ka)

	f.MustReconcile(types.NamespacedName{Name: "a"})
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	var kd v1alpha1.KubernetesDiscovery
	f.MustGet(types.NamespacedName{Name: "a"}, &kd)
	assert.Equal(f.T(), 1, len(kd.Spec.Watches))

	// Make sure we don't contain UID watches
	assert.Equal(t, "", kd.Spec.Watches[0].UID)
	assert.Equal(t, "default", kd.Spec.Watches[0].Namespace)
	assert.Equal(t, map[string]string{"app": "tilt-site"}, kd.Spec.ExtraSelectors[0].MatchLabels)
}

func TestCreateAndDeleteDisco(t *testing.T) {
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
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	var kd v1alpha1.KubernetesDiscovery
	assert.True(f.T(), f.Get(types.NamespacedName{Name: "a"}, &kd))

	f.Delete(&ka)
	f.MustReconcile(types.NamespacedName{Name: "a"})
	assert.False(f.T(), f.Get(types.NamespacedName{Name: "a"}, &kd))
}

func TestDoNotReconcileDiscoOnTransientError(t *testing.T) {
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
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)

	var kd v1alpha1.KubernetesDiscovery
	assert.True(f.T(), f.Get(types.NamespacedName{Name: "a"}, &kd))

	// Simulate a redeploy that fails.
	ka.Spec = v1alpha1.KubernetesApplySpec{
		YAML: strings.Replace(testyaml.SanchoYAML, "name: sancho", "name: sancho2", 1),
	}
	f.kClient.UpsertError = fmt.Errorf("Failed to deploy")
	f.Update(&ka)
	f.MustReconcile(types.NamespacedName{Name: "a"})

	// Assert the KubernetesDeploy is still running.
	f.MustGet(types.NamespacedName{Name: "a"}, &kd)
	assert.Equal(f.T(), 1, len(kd.Spec.Watches))

	// Assert the apply status
	f.MustGet(types.NamespacedName{Name: "a"}, &ka)
	assert.Contains(f.T(), ka.Status.Error, "Failed to deploy")
}
