package indexer

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestListOwnedBy(t *testing.T) {
	c := fake.NewFakeTiltClient()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = logger.WithLogger(ctx, logger.NewTestLogger(os.Stdout))
	scheme := v1alpha1.NewScheme()

	// set up the owner objects
	kd1 := &v1alpha1.KubernetesDiscovery{
		TypeMeta:   metav1.TypeMeta{APIVersion: "tilt.dev/v1alpha1", Kind: "KubernetesDiscovery"},
		ObjectMeta: metav1.ObjectMeta{Name: "fe1"},
	}
	assert.NoError(t, c.Create(ctx, kd1))

	kd2 := &v1alpha1.KubernetesDiscovery{
		TypeMeta:   metav1.TypeMeta{APIVersion: "tilt.dev/v1alpha1", Kind: "KubernetesDiscovery"},
		ObjectMeta: metav1.ObjectMeta{Name: "fe2"},
	}
	assert.NoError(t, c.Create(ctx, kd2))

	// set up the child objects.
	pls1a := &v1alpha1.PodLogStream{
		ObjectMeta: metav1.ObjectMeta{Name: "pls1a"},
		Spec:       v1alpha1.PodLogStreamSpec{Pod: "pls1a"},
	}
	pls1b := &v1alpha1.PodLogStream{
		ObjectMeta: metav1.ObjectMeta{Name: "pls1b"},
		Spec:       v1alpha1.PodLogStreamSpec{Pod: "pls1b"},
	}
	pls2a := &v1alpha1.PodLogStream{
		ObjectMeta: metav1.ObjectMeta{Name: "pls2a"},
		Spec:       v1alpha1.PodLogStreamSpec{Pod: "pls2a"},
	}

	assert.NoError(t, controllerutil.SetControllerReference(kd1, pls1a, scheme))
	assert.NoError(t, controllerutil.SetControllerReference(kd1, pls1b, scheme))
	assert.NoError(t, controllerutil.SetControllerReference(kd2, pls2a, scheme))
	assert.NoError(t, c.Create(ctx, pls1a))
	assert.NoError(t, c.Create(ctx, pls1b))
	assert.NoError(t, c.Create(ctx, pls2a))

	var plsList1 v1alpha1.PodLogStreamList
	assert.NoError(t, ListOwnedBy(ctx, c, &plsList1, types.NamespacedName{Name: kd1.Name}, kd1.TypeMeta))
	assert.ElementsMatch(t, []v1alpha1.PodLogStream{*pls1a, *pls1b}, plsList1.Items)

	var plsList2 v1alpha1.PodLogStreamList
	assert.NoError(t, ListOwnedBy(ctx, c, &plsList2, types.NamespacedName{Name: kd2.Name}, kd2.TypeMeta))
	assert.ElementsMatch(t, []v1alpha1.PodLogStream{*pls2a}, plsList2.Items)
}
