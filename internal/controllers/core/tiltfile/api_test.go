package tiltfile

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
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
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka1))

	err = updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{}}, store.EngineModeUp)
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
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka1))

	err = updateOwnedObjects(ctx, c, nn, tf, &tiltfile.TiltfileLoadResult{
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
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka))
	assert.Contains(t, ka.Spec.YAML, "name: sancho")
	assert.NotContains(t, ka.Spec.YAML, "sidecar")

	fe = manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoSidecarYAML).Build()
	err = updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
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
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	name := apis.SanitizeName(SanchoRef.String())

	var im v1alpha1.ImageMap
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: name}, &im))
	assert.Contains(t, im.Spec.Selector, SanchoRef.String())
}

func TestAPITwoTiltfiles(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()

	feA := manifestbuilder.New(f, "fe-a").WithK8sYAML(testyaml.SanchoYAML).Build()
	nnA := types.NamespacedName{Name: "tiltfile-a"}
	tfA := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile-a"}}

	feB := manifestbuilder.New(f, "fe-b").WithK8sYAML(testyaml.SanchoYAML).Build()
	nnB := types.NamespacedName{Name: "tiltfile-b"}
	tfB := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile-b"}}

	err := updateOwnedObjects(ctx, c, nnA, tfA,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{feA}}, store.EngineModeUp)
	assert.NoError(t, err)

	err = updateOwnedObjects(ctx, c, nnB, tfB,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{feB}}, store.EngineModeUp)
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe-a"}, &ka))
	assert.Contains(t, ka.Name, "fe-a")
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe-b"}, &ka))
	assert.Contains(t, ka.Name, "fe-b")

	err = updateOwnedObjects(ctx, c, nnA, nil, nil, store.EngineModeUp)
	assert.NoError(t, err)

	// Assert that fe-a was deleted but fe-b was not.
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe-b"}, &ka))
	assert.Contains(t, ka.Name, "fe-b")

	err = c.Get(ctx, types.NamespacedName{Name: "fe-a"}, &ka)
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

// Ensure that we create a ConfigMap for each resource's DisableSource
// And set .Spec.DisableSource fields appropriately
func TestCreateDisableSource(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	lr := manifestbuilder.New(f, "be").WithLocalResource("ls", []string{"be"}).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe, lr}}, store.EngineModeUp)
	assert.NoError(t, err)

	feDisable := &v1alpha1.DisableSource{
		ConfigMap: &v1alpha1.ConfigMapDisableSource{
			Name: "fe-disable",
			Key:  "isDisabled",
		},
	}

	var cm v1alpha1.ConfigMap
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: feDisable.ConfigMap.Name}, &cm))
	require.Equal(t, "false", cm.Data[feDisable.ConfigMap.Key])

	name := apis.SanitizeName(SanchoRef.String())
	var im v1alpha1.ImageMap
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: name}, &im))
	require.Equal(t, feDisable, im.Spec.DisableSource)

	var ka v1alpha1.KubernetesApply
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe"}, &ka))
	require.Equal(t, feDisable, ka.Spec.DisableSource)

	beDisable := &v1alpha1.DisableSource{
		ConfigMap: &v1alpha1.ConfigMapDisableSource{
			Name: "be-disable",
			Key:  "isDisabled",
		},
	}

	var fw v1alpha1.FileWatch
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "local:be"}, &fw))
	require.Equal(t, beDisable, fw.Spec.DisableSource)

	var cmd v1alpha1.Cmd
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "be:update"}, &cmd))
	require.Equal(t, beDisable, cmd.Spec.DisableSource)
}

// If a DisableSource ConfigMap already exists, don't replace its data
func TestUpdateDisableSource(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	var cm v1alpha1.ConfigMap
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "fe-disable"}, &cm))
	cm.Data["isDisabled"] = "true"
	require.NoError(t, c.Update(ctx, &cm))

	err = updateOwnedObjects(ctx, c, nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}}, store.EngineModeUp)
	assert.NoError(t, err)

	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: cm.Name}, &cm))
	require.Equal(t, "true", cm.Data["isDisabled"])
}
