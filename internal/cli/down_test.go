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
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDown(t *testing.T) {
	f := newDownFixture(t)
	defer f.TearDown()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newK8sManifest()}
	err := f.cmd.down(f.ctx, f.deps, nil)
	assert.NoError(t, err)
	assert.Contains(t, f.kCli.DeletedYaml, "sancho")
}

func TestDownPreservesNamespacesByDefault(t *testing.T) {
	f := newDownFixture(t)
	defer f.TearDown()

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
	defer f.TearDown()

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

func TestDownK8sFails(t *testing.T) {
	f := newDownFixture(t)
	defer f.TearDown()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newK8sManifest()}
	f.kCli.DeleteError = fmt.Errorf("GARBLEGARBLE")
	err := f.cmd.down(f.ctx, f.deps, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "GARBLEGARBLE")
	}
}

func TestDownDCFails(t *testing.T) {
	f := newDownFixture(t)
	defer f.TearDown()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Manifests: newDCManifest()}
	f.dcc.DownError = fmt.Errorf("GARBLEGARBLE")
	err := f.cmd.down(f.ctx, f.deps, nil)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "GARBLEGARBLE")
	}
}

func TestDownArgs(t *testing.T) {
	f := newDownFixture(t)
	defer f.TearDown()

	cmd := f.cmd.register()
	cmd.SetArgs([]string{"foo", "bar"})
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx, _, _ := testutils.CtxAndAnalyticsForTest()
		err := f.cmd.run(ctx, args)
		require.NoError(t, err)
	}
	err := cmd.Execute()
	require.NoError(t, err)

	require.Equal(t, []string{"foo", "bar"}, f.tfl.PassedUserConfigState().Args)
}

func newK8sManifest() []model.Manifest {
	return []model.Manifest{model.Manifest{Name: "fe"}.WithDeployTarget(k8s.MustTarget("fe", testyaml.SanchoYAML))}
}

func newDCManifest() []model.Manifest {
	return []model.Manifest{model.Manifest{Name: "fe"}.WithDeployTarget(model.DockerComposeTarget{
		Name:        "fe",
		ConfigPaths: []string{"dc.yaml"},
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
	return model.Manifest{Name: model.ManifestName(name)}.WithDeployTarget(model.K8sTarget{YAML: yaml})
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
}

func newDownFixture(t *testing.T) downFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	tfl := tiltfile.NewFakeTiltfileLoader()
	dcc := dockercompose.NewFakeDockerComposeClient(t, ctx)
	kCli := k8s.NewFakeK8sClient()
	downDeps := DownDeps{tfl, dcc, kCli}
	cmd := &downCmd{downDepsProvider: func(ctx context.Context, tiltAnalytics *analytics.TiltAnalytics) (deps DownDeps, err error) {
		return downDeps, nil
	}}
	return downFixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		cmd:    cmd,
		deps:   downDeps,
		tfl:    tfl,
		dcc:    dcc,
		kCli:   kCli,
	}
}

func (f *downFixture) TearDown() {
	f.cancel()
}
