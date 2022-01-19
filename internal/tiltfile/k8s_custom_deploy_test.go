package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestK8sCustomDeployLiveUpdateImageSelector(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.skipYAML = true
	f.tiltfileCode = `
default_registry('gcr.io/myrepo')
k8s_custom_deploy('foo', 'apply', 'delete', deps=['foo'], image_selector='foo-img', live_update=%s)
`
	f.init()

	f.load("foo")

	m := f.assertNextManifest("foo", cb(image("foo-img"), f.expectedLU))
	assert.True(t, m.ImageTargets[0].IsLiveUpdateOnly)
	// this ref will never actually be used since the image isn't being built but the registry is applied here
	assert.Equal(t, "gcr.io/myrepo/foo-img", m.ImageTargets[0].Refs.LocalRef().String())

	require.NoError(t, m.InferLiveUpdateSelectors(), "Failed to infer Live Update selectors")
	luSpec := m.ImageTargets[0].LiveUpdateSpec
	require.NotNil(t, luSpec.Selector.Kubernetes)
	assert.Empty(t, luSpec.Selector.Kubernetes.ContainerName)
	// NO registry rewriting should be applied here because Tilt isn't actually building the image
	assert.Equal(t, "foo-img", luSpec.Selector.Kubernetes.Image)
}

func TestK8sCustomDeployLiveUpdateContainerNameSelector(t *testing.T) {
	f := newLiveUpdateFixture(t)
	defer f.TearDown()

	f.skipYAML = true
	f.tiltfileCode = `
k8s_custom_deploy('foo', 'apply', 'delete', deps=['foo'], container_selector='bar', live_update=%s)
`
	f.init()

	f.load("foo")
	f.expectedLU.Selector.Kubernetes = &v1alpha1.LiveUpdateKubernetesSelector{
		ContainerName: "bar",
	}

	// NOTE: because there is no known image name, the manifest name is used to
	// 	generate one since an image target without a ref is not valid
	m := f.assertNextManifest("foo", cb(image("k8s_custom_deploy:foo"), f.expectedLU))
	assert.True(t, m.ImageTargets[0].IsLiveUpdateOnly)

	require.NoError(t, m.InferLiveUpdateSelectors(), "Failed to infer Live Update selectors")
	luSpec := m.ImageTargets[0].LiveUpdateSpec
	require.NotNil(t, luSpec.Selector.Kubernetes)
	assert.Empty(t, luSpec.Selector.Kubernetes.Image)
	// NO registry rewriting should be applied here because Tilt isn't actually building the image
	assert.Equal(t, "bar", luSpec.Selector.Kubernetes.ContainerName)
}

func TestK8sCustomDeployNoLiveUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  apply_dir='apply-dir',
                  delete_dir='delete-dir',
                  apply_env={'APPLY_KEY': '1'},
                  delete_env={'DELETE_KEY': 'baz'},
                  deps=['foo'])
`)

	f.load("foo")

	m := f.assertNextManifest("foo")
	assert.Empty(t, m.ImageTargets, "No image targets should have been created")

	spec := m.K8sTarget().KubernetesApplySpec
	assertK8sApplyCmdEqual(f,
		model.ToHostCmdInDirWithEnv("apply", "apply-dir", []string{"APPLY_KEY=1"}),
		spec.ApplyCmd)
	assertK8sApplyCmdEqual(f,
		model.ToHostCmdInDirWithEnv("delete", "delete-dir", []string{"DELETE_KEY=baz"}),
		spec.DeleteCmd)
}

func TestK8sCustomDeployImageDepsMissing(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[],
                  image_deps=['image-a'])
`)

	f.loadErrString(`resource "foo": image build "image-a" not found`)
}

func TestK8sCustomDeployImageDepsMalformed(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[],
                  image_deps=['image a'])
`)

	f.loadErrString(`k8s_custom_deploy: for parameter "image_deps": must be a valid image reference: invalid reference format`)
}

func TestK8sCustomDeployImageDepExists(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Dockerfile", "FROM golang:1.10")
	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[],
                  image_deps=['image-a'])

docker_build('image-a', '.')
`)

	f.load()
	m := f.assertNextManifest("foo")
	assert.Equal(t, 1, len(m.ImageTargets))

	spec := m.K8sTarget().KubernetesApplySpec
	assert.Equal(t, []string{"image-a"}, spec.ImageMaps)
}

func TestK8sCustomDeployConflict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[])
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[])
`)

	f.loadErrString(`k8s_resource named "foo" already exists`)
}

func TestK8sCustomDeployLocalResourceConflict(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
k8s_custom_deploy('foo',
                  apply_cmd='apply',
                  delete_cmd='delete',
                  deps=[])
local_resource('foo', 'foo')
`)

	f.loadErrString(`k8s_resource named "foo" already exists`)
}

func assertK8sApplyCmdEqual(f *fixture, expected model.Cmd, actual *v1alpha1.KubernetesApplyCmd) bool {
	t := f.t
	t.Helper()
	if !assert.NotNil(t, actual, "KubernetesApplyCmd was nil") {
		return false
	}
	result := true
	result = assert.Equal(t, expected.Argv, actual.Args, "Args were not equal") && result
	result = assert.Equal(t, expected.Env, actual.Env, "Env was not equal") && result
	result = assert.Equal(t, f.JoinPath(expected.Dir), actual.Dir, "Working dir was not equal") && result
	return result
}
