package cli

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDownK8sYAML(t *testing.T) {
	f := newDownFixture(t)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newK8sManifest()}
	err := f.cmd.down(f.ctx, f.deps, nil)
	assert.NoError(t, err)
	assert.Contains(t, f.kCli.DeletedYaml, "sancho")
}

func TestDownPreservesEntitiesWithKeepLabel(t *testing.T) {
	f := newDownFixture(t)

	manifests := append([]model.Manifest{}, newK8sPVCManifest("foo", "keep"), newK8sPVCManifest("bar", "delete"))

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)
	require.Contains(t, f.kCli.DeletedYaml, "bar")
	require.NotContains(t, f.kCli.DeletedYaml, "foo")
}

func TestDownPreservesNamespacesByDefault(t *testing.T) {
	f := newDownFixture(t)

	manifests := append([]model.Manifest{}, newK8sManifest()...)
	manifests = append(manifests, newK8sNamespaceManifest("foo"), newK8sNamespaceManifest("bar"))

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)
	require.Contains(t, f.kCli.DeletedYaml, "sancho")
	for _, ns := range []string{"foo", "bar"} {
		require.NotContains(t, f.kCli.DeletedYaml, ns)
	}
}

func TestDownDeletesNamespacesIfSpecified(t *testing.T) {
	f := newDownFixture(t)

	manifests := append([]model.Manifest{}, newK8sManifest()...)
	manifests = append(manifests, newK8sNamespaceManifest("foo"), newK8sNamespaceManifest("bar"))

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	f.cmd.deleteNamespaces = true
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)
	for _, ns := range []string{"sancho", "foo", "bar"} {
		require.Contains(t, f.kCli.DeletedYaml, ns)
	}
}

func TestDownDeletesManifestsInReverseOrder(t *testing.T) {
	f := newDownFixture(t)

	manifests := append([]model.Manifest{}, newK8sNamespaceManifest("foo"))
	manifests = append(manifests, newK8sManifest()...)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	f.cmd.deleteNamespaces = true
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)
	require.Regexp(t, "(?s)name: sancho.*name: foo", f.kCli.DeletedYaml) // namespace comes after deployment
}

func TestDownDeletesEntitiesInReverseOrder(t *testing.T) {
	f := newDownFixture(t)

	manifests := []model.Manifest{newK8sMultiEntityManifest()}

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	f.cmd.deleteNamespaces = true
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)

	entities, err := k8s.ParseYAMLFromString(f.kCli.DeletedYaml)
	require.NoError(t, err)
	require.Equal(t, 2, len(entities))
	require.Equal(t, "Secret", entities[0].GVK().Kind)
	require.Equal(t, "Namespace", entities[1].GVK().Kind)
}

func TestDownDeletesInDependentOrder(t *testing.T) {
	f := newDownFixture(t)

	manifests := newK8sDependentManifests()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)

	entities, err := k8s.ParseYAMLFromString(f.kCli.DeletedYaml)
	require.NoError(t, err)
	require.Equal(t, 6, len(entities))

	var names []string

	for _, entity := range entities {
		names = append(names, entity.Meta().GetName())
	}

	// For each name with dependencies, assert that its dependencies are deleted after it
	for i, name := range names {
		switch name {
		case "mixed_dependent":
			require.Contains(t, names[i:], "no_dependencies")
			require.Contains(t, names[i:], "direct_dependent_1")
			require.Contains(t, names[i:], "indirect_dependent_2")
		case "indirect_dependent_1":
			require.Contains(t, names[i:], "direct_dependent_2")
		case "indirect_dependent_2":
			require.Contains(t, names[i:], "direct_dependent_1")
		case "direct_dependent_1":
			require.Contains(t, names[i:], "no_dependencies")
		case "direct_dependent_2":
			require.Contains(t, names[i:], "no_dependencies")
		}
	}
}

func TestDownDeletesInDependentOrderReversed(t *testing.T) {
	f := newDownFixture(t)

	manifests := newK8sDependentManifests()

	// Reverse the list of manifests to ensure delete order is dependent on manifest order
	for i := 0; i < len(manifests)/2; i++ {
		manifests[i], manifests[len(manifests)-i-1] = manifests[len(manifests)-i-1], manifests[i]
	}

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)

	entities, err := k8s.ParseYAMLFromString(f.kCli.DeletedYaml)
	require.NoError(t, err)
	require.Equal(t, 6, len(entities))

	var names []string

	for _, entity := range entities {
		names = append(names, entity.Meta().GetName())
	}

	// For each name with dependencies, assert that its dependencies are deleted after it
	for i, name := range names {
		switch name {
		case "mixed_dependent":
			require.Contains(t, names[i:], "no_dependencies")
			require.Contains(t, names[i:], "direct_dependent_1")
			require.Contains(t, names[i:], "indirect_dependent_2")
		case "indirect_dependent_1":
			require.Contains(t, names[i:], "direct_dependent_2")
		case "indirect_dependent_2":
			require.Contains(t, names[i:], "direct_dependent_1")
		case "direct_dependent_1":
			require.Contains(t, names[i:], "no_dependencies")
		case "direct_dependent_2":
			require.Contains(t, names[i:], "no_dependencies")
		}
	}
}

func TestDownDeletesCyclicDependencies(t *testing.T) {
	f := newDownFixture(t)

	manifests := newK8sCyclicManifest()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)

	entities, err := k8s.ParseYAMLFromString(f.kCli.DeletedYaml)
	require.NoError(t, err)

	require.Equal(t, 2, len(entities))
}

func TestDownK8sFails(t *testing.T) {
	f := newDownFixture(t)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newK8sManifest()}
	f.kCli.DeleteError = fmt.Errorf("GARBLEGARBLE")
	err := f.cmd.down(f.ctx, f.deps, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "GARBLEGARBLE")
	}
}

func TestDownK8sDeleteCmd(t *testing.T) {
	f := newDownFixture(t)

	kaSpec := v1alpha1.KubernetesApplySpec{
		ApplyCmd:  &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-deploy-cmd"}},
		DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
	}

	kt, err := k8s.NewTarget("fe", kaSpec, model.PodReadinessIgnore, nil)
	require.NoError(t, err, "Failed to make KubernetesTarget")
	m := model.Manifest{Name: "fe"}.WithDeployTarget(kt)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{m}}
	err = f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)

	calls := f.execer.Calls()
	if assert.Len(t, calls, 1, "Should have been exactly 1 exec call") {
		assert.Equal(t, []string{"custom-delete-cmd"}, calls[0].Cmd.Argv)
	}
}

func TestDownK8sDeleteCmd_Error(t *testing.T) {
	f := newDownFixture(t)

	f.execer.RegisterCommand("custom-delete-cmd", 321, "", "delete failed")

	kaSpec := v1alpha1.KubernetesApplySpec{
		ApplyCmd:  &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-deploy-cmd"}},
		DeleteCmd: &v1alpha1.KubernetesApplyCmd{Args: []string{"custom-delete-cmd"}},
	}

	kt, err := k8s.NewTarget("fe", kaSpec, model.PodReadinessIgnore, nil)
	require.NoError(t, err, "Failed to make KubernetesTarget")
	m := model.Manifest{Name: "fe"}.WithDeployTarget(kt)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: []model.Manifest{m}}
	err = f.cmd.down(f.ctx, f.deps, nil)
	assert.EqualError(t, err, "Deleting k8s entities for cmd: custom-delete-cmd: exit status 321")

	calls := f.execer.Calls()
	if assert.Len(t, calls, 1, "Should have been exactly 1 exec call") {
		assert.Equal(t, []string{"custom-delete-cmd"}, calls[0].Cmd.Argv)
	}
}

func TestDownDCFails(t *testing.T) {
	f := newDownFixture(t)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newDCManifest()}
	f.dcc.DownError = fmt.Errorf("GARBLEGARBLE")
	err := f.cmd.down(f.ctx, f.deps, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "GARBLEGARBLE")
	}
}

func TestDownArgs(t *testing.T) {
	f := newDownFixture(t)

	cmd := f.cmd.register()
	cmd.SetArgs([]string{"foo", "bar"})
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx, _, _ := testutils.CtxAndAnalyticsForTest()
		err := f.cmd.run(ctx, args)
		require.NoError(t, err)
	}
	err := cmd.Execute()
	require.NoError(t, err)

	require.Equal(t, []string{"foo", "bar"}, f.tfl.PassedArgs())
}

func newK8sManifest() []model.Manifest {
	return []model.Manifest{model.Manifest{Name: "fe"}.WithDeployTarget(k8s.MustTarget("fe", testyaml.SanchoYAML))}
}

func newK8sDependentManifests() []model.Manifest {
	yamlTemplate := `
apiVersion: v1
kind: Secret
metadata:
  name: %s
data:
  mySecret: blah
`

	return []model.Manifest{
		model.Manifest{
			Name: "no_dependencies",
		}.WithDeployTarget(k8s.MustTarget("no_dependencies", fmt.Sprintf(yamlTemplate, "no_dependencies"))),
		model.Manifest{
			Name:                 "direct_dependent_1",
			ResourceDependencies: []model.ManifestName{"no_dependencies"},
		}.WithDeployTarget(k8s.MustTarget("direct_dependent_1", fmt.Sprintf(yamlTemplate, "direct_dependent_1"))),
		model.Manifest{
			Name:                 "direct_dependent_2",
			ResourceDependencies: []model.ManifestName{"no_dependencies"},
		}.WithDeployTarget(k8s.MustTarget("direct_dependent_2", fmt.Sprintf(yamlTemplate, "direct_dependent_2"))),
		model.Manifest{
			Name:                 "indirect_dependent_1",
			ResourceDependencies: []model.ManifestName{"direct_dependent_2"},
		}.WithDeployTarget(k8s.MustTarget("indirect_dependent_1", fmt.Sprintf(yamlTemplate, "indirect_dependent_1"))),
		model.Manifest{
			Name:                 "indirect_dependent_2",
			ResourceDependencies: []model.ManifestName{"direct_dependent_1"},
		}.WithDeployTarget(k8s.MustTarget("indirect_dependent_2", fmt.Sprintf(yamlTemplate, "indirect_dependent_2"))),
		model.Manifest{
			Name:                 "mixed_dependent",
			ResourceDependencies: []model.ManifestName{"no_dependencies", "direct_dependent_1", "indirect_dependent_2"},
		}.WithDeployTarget(k8s.MustTarget("mixed_dependent", fmt.Sprintf(yamlTemplate, "mixed_dependent"))),
	}
}

func newK8sCyclicManifest() []model.Manifest {
	yamlTemplate := `
apiVersion: v1
kind: Secret
metadata:
  name: %s
data:
  mySecret: blah
`

	return []model.Manifest{
		model.Manifest{
			Name:                 "dep_1",
			ResourceDependencies: []model.ManifestName{"dep_2"},
		}.WithDeployTarget(k8s.MustTarget("dep_1", fmt.Sprintf(yamlTemplate, "dep_1"))),
		model.Manifest{
			Name:                 "dep_2",
			ResourceDependencies: []model.ManifestName{"dep_1"},
		}.WithDeployTarget(k8s.MustTarget("dep_2", fmt.Sprintf(yamlTemplate, "dep_2"))),
	}
}

func newDCManifest() []model.Manifest {
	return []model.Manifest{model.Manifest{Name: "fe"}.WithDeployTarget(model.DockerComposeTarget{
		Name: "fe",
		Spec: v1alpha1.DockerComposeServiceSpec{
			Service: "fe",
			Project: v1alpha1.DockerComposeProject{
				ConfigPaths: []string{"dc.yaml"},
			},
		},
	})}
}

func newK8sMultiEntityManifest() model.Manifest {
	yaml := `
apiVersion: v1
kind: Namespace
metadata:
  name: test-namespace
---
apiVersion: v1
kind: Secret
metadata:
  name: test-secret
  namespace: test-namespace
data:
  testSecret: blah
`

	return model.Manifest{Name: "test-secret"}.WithDeployTarget(k8s.MustTarget("test-secret", yaml))
}

func newK8sNamespaceManifest(name string) model.Manifest {
	yaml := fmt.Sprintf(`
apiVersion: v1
kind: Namespace
metadata:
  name: %s
spec: {}
status: {}`, name)
	return model.Manifest{Name: model.ManifestName(name)}.WithDeployTarget(model.NewK8sTargetForTesting(yaml))
}

func newK8sPVCManifest(name string, downPolicy string) model.Manifest {
	yaml := fmt.Sprintf(`
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: %s
  annotations:
    tilt.dev/down-policy: %s
spec: {}
status: {}`, name, downPolicy)
	return model.Manifest{Name: model.ManifestName(name)}.WithDeployTarget(model.NewK8sTargetForTesting(yaml))
}

type downFixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	cmd    *downCmd
	deps   DownDeps
	tfl    *tiltfile.FakeTiltfileLoader
	dcc    *dockercompose.FakeDCClient
	kCli   *k8s.FakeK8sClient
	execer *localexec.FakeExecer
}

func newDownFixture(t *testing.T) downFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	tfl := tiltfile.NewFakeTiltfileLoader()
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kCli := k8s.NewFakeK8sClient(t)
	execer := localexec.NewFakeExecer(t)
	downDeps := DownDeps{tfl, dcc, kCli, execer}
	cmd := &downCmd{downDepsProvider: func(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics, subcommand model.TiltSubcommand) (deps DownDeps, err error) {
		return downDeps, nil
	}}
	ret := downFixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
		deps:   downDeps,
		tfl:    tfl,
		dcc:    dcc,
		kCli:   kCli,
		execer: execer,
	}

	t.Cleanup(ret.TearDown)

	return ret
}

func (f *downFixture) TearDown() {
	f.cancel()
}
