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

func TestDownDeletesInReverseOrder(t *testing.T) {
	f := newDownFixture(t)

	manifests := append([]model.Manifest{}, newK8sNamespaceManifest("foo"))
	manifests = append(manifests, newK8sManifest()...)

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: manifests}
	f.cmd.deleteNamespaces = true
	err := f.cmd.down(f.ctx, f.deps, nil)
	require.NoError(t, err)
	require.Regexp(t, "(?s)name: sancho.*name: foo", f.kCli.DeletedYaml) // namespace comes after deployment
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
