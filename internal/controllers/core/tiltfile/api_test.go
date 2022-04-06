package tiltfile

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/configmap"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestAPICreate(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka))
	assert.Contains(t, ka.Spec.YAML, "name: sancho")
}

func TestAPIDelete(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka1))

	err = f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{}})
	assert.NoError(t, err)

	var ka2 v1alpha1.KubernetesApply
	err = f.Get(types.NamespacedName{Name: "fe"}, &ka2)
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

func TestAPINoGarbageCollectOnError(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	var ka1 v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka1))

	err = f.updateOwnedObjects(nn, tf, &tiltfile.TiltfileLoadResult{
		Error:     fmt.Errorf("random failure"),
		Manifests: []model.Manifest{},
	})
	assert.NoError(t, err)

	var ka2 v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka2))
	assert.Equal(t, ka1, ka2)
}

func TestAPIUpdate(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka))
	assert.Contains(t, ka.Spec.YAML, "name: sancho")
	assert.NotContains(t, ka.Spec.YAML, "sidecar")

	fe = manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoSidecarYAML).Build()
	err = f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	err = f.Get(types.NamespacedName{Name: "fe"}, &ka)
	assert.NoError(t, err)
	assert.Contains(t, ka.Spec.YAML, "sidecar")
}

func TestImageMapCreate(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	name := apis.SanitizeName(SanchoRef.String())

	var im v1alpha1.ImageMap
	assert.NoError(t, f.Get(types.NamespacedName{Name: name}, &im))
	assert.Contains(t, im.Spec.Selector, SanchoRef.String())

	diName := apis.SanitizeName(fmt.Sprintf("fe:%s", SanchoRef.String()))
	var di v1alpha1.DockerImage
	assert.NoError(t, f.Get(types.NamespacedName{Name: diName}, &di))
	assert.Contains(t, di.Spec.Ref, SanchoRef.String())
}

func TestCmdImageCreate(t *testing.T) {
	f := newAPIFixture(t)
	target := model.MustNewImageTarget(SanchoRef).
		WithBuildDetails(model.CustomBuild{
			CmdImageSpec: v1alpha1.CmdImageSpec{Args: []string{"echo"}},
			Deps:         []string{f.Path()},
		})
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(target).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	name := apis.SanitizeName(SanchoRef.String())

	var im v1alpha1.ImageMap
	assert.NoError(t, f.Get(types.NamespacedName{Name: name}, &im))
	assert.Contains(t, im.Spec.Selector, SanchoRef.String())

	ciName := apis.SanitizeName(fmt.Sprintf("fe:%s", SanchoRef.String()))
	var ci v1alpha1.CmdImage
	assert.NoError(t, f.Get(types.NamespacedName{Name: ciName}, &ci))
	assert.Contains(t, ci.Spec.Ref, SanchoRef.String())
}

func TestTwoManifestsShareImage(t *testing.T) {
	f := newAPIFixture(t)
	target := model.MustNewImageTarget(SanchoRef).
		WithBuildDetails(model.CustomBuild{
			CmdImageSpec: v1alpha1.CmdImageSpec{Args: []string{"echo"}},
			Deps:         []string{f.Path()},
		})
	fe1 := manifestbuilder.New(f, "fe1").
		WithImageTarget(target).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	fe2 := manifestbuilder.New(f, "fe2").
		WithImageTarget(target).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe1, fe2}})
	assert.NoError(t, err)

	name := apis.SanitizeName(fe1.ImageTargets[0].ID().String())

	var fw v1alpha1.FileWatch
	assert.NoError(t, f.Get(types.NamespacedName{Name: name}, &fw))
	assert.Equal(t, fw.Spec.DisableSource, &v1alpha1.DisableSource{
		EveryConfigMap: []v1alpha1.ConfigMapDisableSource{
			{Name: "fe1-disable", Key: "isDisabled"},
			{Name: "fe2-disable", Key: "isDisabled"},
		},
	})
}

func TestAPITwoTiltfiles(t *testing.T) {
	f := newAPIFixture(t)
	feA := manifestbuilder.New(f, "fe-a").WithK8sYAML(testyaml.SanchoYAML).Build()
	nnA := types.NamespacedName{Name: "tiltfile-a"}
	tfA := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile-a"}}

	feB := manifestbuilder.New(f, "fe-b").WithK8sYAML(testyaml.SanchoYAML).Build()
	nnB := types.NamespacedName{Name: "tiltfile-b"}
	tfB := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile-b"}}

	err := f.updateOwnedObjects(nnA, tfA,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{feA}})
	assert.NoError(t, err)

	err = f.updateOwnedObjects(nnB, tfB,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{feB}})
	assert.NoError(t, err)

	var ka v1alpha1.KubernetesApply
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe-a"}, &ka))
	assert.Contains(t, ka.Name, "fe-a")
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe-b"}, &ka))
	assert.Contains(t, ka.Name, "fe-b")

	err = f.updateOwnedObjects(nnA, nil, nil)
	assert.NoError(t, err)

	// Assert that fe-a was deleted but fe-b was not.
	assert.NoError(t, f.Get(types.NamespacedName{Name: "fe-b"}, &ka))
	assert.Contains(t, ka.Name, "fe-b")

	err = f.Get(types.NamespacedName{Name: "fe-a"}, &ka)
	if assert.Error(t, err) {
		assert.True(t, apierrors.IsNotFound(err))
	}
}

func TestCreateUiResourceForTiltfile(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	lr := manifestbuilder.New(f, "be").WithLocalResource("ls", []string{"be"}).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile", Labels: map[string]string{"some": "sweet-label"}}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe, lr}})
	assert.NoError(t, err)

	var uir v1alpha1.UIResource
	require.NoError(t, f.Get(types.NamespacedName{Name: "tiltfile"}, &uir))
	require.Equal(t, map[string]string{"some": "sweet-label"}, uir.ObjectMeta.Labels)
	require.Equal(t, "tiltfile", uir.ObjectMeta.Name)
}

func TestCreateClusterDefaultRegistry(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	tf := &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: model.MainTiltfileManifestName.String()},
	}
	nn := apis.Key(tf)
	reg := container.MustNewRegistry("registry.example.com")
	reg.SingleName = "fake-repo"
	tlr := &tiltfile.TiltfileLoadResult{
		Manifests:       []model.Manifest{fe},
		DefaultRegistry: reg,
	}
	err := f.updateOwnedObjects(nn, tf, tlr)
	assert.NoError(t, err)

	var cluster v1alpha1.Cluster
	require.NoError(t, f.Get(types.NamespacedName{Name: "default"}, &cluster))
	require.NotNil(t, cluster.Spec.DefaultRegistry, ".Spec.DefaultRegistry was nil")
	require.Equal(t, "registry.example.com", cluster.Spec.DefaultRegistry.Host, "Default registry host")
	require.Equal(t, "fake-repo", cluster.Spec.DefaultRegistry.SingleName, "Default registry single name")
}

// Ensure that we emit disable-related objects/field appropriately
func TestDisableObjects(t *testing.T) {
	for _, disableFeatureOn := range []bool{true, false} {
		t.Run(fmt.Sprintf("disable buttons enabled: %v", disableFeatureOn), func(t *testing.T) {
			f := newAPIFixture(t)
			fe := manifestbuilder.New(f, "fe").
				WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
				WithK8sYAML(testyaml.SanchoYAML).
				Build()
			lr := manifestbuilder.New(f, "be").WithLocalResource("ls", []string{"be"}).Build()
			nn := types.NamespacedName{Name: "tiltfile"}
			tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
			err := f.updateOwnedObjects(nn, tf,
				&tiltfile.TiltfileLoadResult{
					Manifests:    []model.Manifest{fe, lr},
					FeatureFlags: map[string]bool{feature.DisableResources: disableFeatureOn},
				})
			assert.NoError(t, err)

			feDisable := &v1alpha1.DisableSource{
				ConfigMap: &v1alpha1.ConfigMapDisableSource{
					Name: "fe-disable",
					Key:  "isDisabled",
				},
			}

			var cm v1alpha1.ConfigMap
			require.NoError(t, f.Get(types.NamespacedName{Name: feDisable.ConfigMap.Name}, &cm))
			require.Equal(t, "true", cm.Data[feDisable.ConfigMap.Key])

			name := apis.SanitizeName(SanchoRef.String())
			var im v1alpha1.ImageMap
			require.NoError(t, f.Get(types.NamespacedName{Name: name}, &im))

			var ka v1alpha1.KubernetesApply
			require.NoError(t, f.Get(types.NamespacedName{Name: "fe"}, &ka))
			require.Equal(t, feDisable, ka.Spec.DisableSource)

			beDisable := &v1alpha1.DisableSource{
				ConfigMap: &v1alpha1.ConfigMapDisableSource{
					Name: "be-disable",
					Key:  "isDisabled",
				},
			}

			var fw v1alpha1.FileWatch
			require.NoError(t, f.Get(types.NamespacedName{Name: "local:be"}, &fw))
			require.Equal(t, beDisable, fw.Spec.DisableSource)

			var cmd v1alpha1.Cmd
			require.NoError(t, f.Get(types.NamespacedName{Name: "be:update"}, &cmd))
			require.Equal(t, beDisable, cmd.Spec.DisableSource)

			var uir v1alpha1.UIResource
			require.NoError(t, f.Get(types.NamespacedName{Name: "be"}, &uir))
			require.Equal(t, []v1alpha1.DisableSource{*beDisable}, uir.Status.DisableStatus.Sources)

			var tb v1alpha1.ToggleButton
			err = f.Get(types.NamespacedName{Name: "fe-disable"}, &tb)
			if disableFeatureOn {
				require.NoError(t, err)
				require.Equal(t, feDisable.ConfigMap.Name, tb.Spec.StateSource.ConfigMap.Name)
			} else {
				require.True(t, apierrors.IsNotFound(err))
			}

			err = f.Get(types.NamespacedName{Name: "be-disable"}, &tb)
			if disableFeatureOn {
				require.NoError(t, err)
				require.Equal(t, beDisable.ConfigMap.Name, tb.Spec.StateSource.ConfigMap.Name)
			} else {
				require.True(t, apierrors.IsNotFound(err))
			}
		})
	}
}

// If a DisableSource ConfigMap already exists, don't replace its data
func TestUpdateDisableSource(t *testing.T) {
	f := newAPIFixture(t)
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	err := f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	err = configmap.UpsertDisableConfigMap(f.ctx, f.c, "fe-disable", "isDisabled", true)
	require.NoError(t, err)

	err = f.updateOwnedObjects(nn, tf,
		&tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}})
	assert.NoError(t, err)

	var cm v1alpha1.ConfigMap
	require.NoError(t, f.Get(types.NamespacedName{Name: "fe-disable"}, &cm))
	require.Equal(t, "true", cm.Data["isDisabled"])
}

// make sure that objects created by the Tiltfile are included in typesToReconcile, so that
// they get cleaned up when they go away
// note: this test is not exhaustive, since not all branches generate all types that are possibly
// generated by a Tiltfile, but hopefully it at least catches most common cases
func TestReconciledTypesCompleteness(t *testing.T) {
	f := newAPIFixture(t)
	nn := types.NamespacedName{Name: "tiltfile"}
	tf := &v1alpha1.Tiltfile{ObjectMeta: metav1.ObjectMeta{Name: "tiltfile"}}
	fe := manifestbuilder.New(f, "fe").WithK8sYAML(testyaml.SanchoYAML).Build()
	tlr := &tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{fe}, FeatureFlags: map[string]bool{feature.DisableResources: true}}
	ds := toDisableSources(tlr)
	objs := toAPIObjects(nn, tf, tlr, store.EngineModeCI, &v1alpha1.KubernetesClusterConnection{}, ds)

	reconciledTypes := make(map[schema.GroupVersionResource]bool)
	for _, t := range typesToReconcile {
		reconciledTypes[t.GetGroupVersionResource()] = true
	}

	for _, os := range objs {
		for _, v := range os {
			require.Truef(t,
				reconciledTypes[v.GetGroupVersionResource()],
				"object %q of type %q was generated by the Tiltfile, but is not listed in typesToReconcile.\n"+
					"either add the type to typesToReconcile or change the Tiltfile reconciler to not generate it.",
				v.GetName(),
				v.GetGroupVersionResource())
		}
	}
}

type apiFixture struct {
	ctx context.Context
	c   ctrlclient.Client
	*tempdir.TempDirFixture
}

func newAPIFixture(t testing.TB) *apiFixture {
	f := tempdir.NewTempDirFixture(t)

	ctx := context.Background()
	c := fake.NewFakeTiltClient()
	return &apiFixture{
		ctx:            ctx,
		c:              c,
		TempDirFixture: f,
	}
}

func (f *apiFixture) updateOwnedObjects(nn types.NamespacedName, tf *v1alpha1.Tiltfile, tlr *tiltfile.TiltfileLoadResult) error {
	return updateOwnedObjects(f.ctx, f.c, nn, tf, tlr, false, store.EngineModeUp,
		&v1alpha1.KubernetesClusterConnection{})
}

func (f *apiFixture) Get(nn types.NamespacedName, obj ctrlclient.Object) error {
	return f.c.Get(f.ctx, nn, obj)
}
