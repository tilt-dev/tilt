package configs

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestAPICreate(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	err := updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka))
	assert.Contains(t, ka.Spec.YAML, "name: sancho")
}

func TestAPIDelete(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	err := updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka1))

	err = updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka2 v1alpha1.KubernetesApply
	err = c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka2)
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

func TestAPINoGarbageCollectOnError(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	err := updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka1))

	err = updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{
		Error:     fmt.Errorf("random failure"),
		Manifests: []model.Manifest{},
	}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka2 v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka2))
	assert.Equal(t, ka1, ka2)
}

func TestAPIUpdate(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	err := updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka))
	assert.Contains(t, ka.Spec.YAML, "name: sancho")
	assert.NotContains(t, ka.Spec.YAML, "sidecar")

	fe = manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoSidecarYAML).Build()
	err = updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	err = c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka)
	assert.NoError(t, err)
	assert.Contains(t, ka.Spec.YAML, "sidecar")
}

func TestImageMapCreate(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	err := updateOwnedObjects(ctx, c, tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	name := apis.SanitizeName(SanchoRef.String())

	var im v1alpha1.ImageMap
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: name}, &im))
	assert.Contains(t, im.Spec.Selector, SanchoRef.String())
}
