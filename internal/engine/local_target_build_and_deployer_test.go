package engine

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestNoLocalTargets(t *testing.T) {
	f := newLTFixture()

	specs := []model.TargetSpec{
		model.ImageTarget{}, model.K8sTarget{}, model.DockerComposeTarget{},
	}
	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, specs, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty result for failed BuildAndDeploy")

	require.NotNil(t, err)
	assert.Contains(t, err.Error(),
		"LocalTargetBuildAndDeployer requires exactly one LocalTarget (got 0)")
}

func TestTooManyLocalTargets(t *testing.T) {
	f := newLTFixture()

	specs := []model.TargetSpec{
		model.LocalTarget{}, model.ImageTarget{}, model.K8sTarget{}, model.LocalTarget{},
	}
	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, specs, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty result for failed BuildAndDeploy")

	require.NotNil(t, err)
	assert.Contains(t, err.Error(),
		"LocalTargetBuildAndDeployer requires exactly one LocalTarget (got 2)")
}

func TestSuccessfulCommand(t *testing.T) {
	f := newLTFixture()
	targ := model.LocalTarget{Cmd: model.ToShellCmd("echo hello world")}

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, targ.ID(), res[targ.ID()].TargetID)

	assert.Contains(t, f.out.String(), "hello world", "expect cmd stdout in logs")
}

func TestExtractOneLocalTarget(t *testing.T) {
	f := newLTFixture()
	targ := model.LocalTarget{Cmd: model.ToShellCmd("echo hello world")}

	// Even if there are multiple other targets, should correctly extract and run the one LocalTarget
	specs := []model.TargetSpec{
		targ, model.ImageTarget{}, model.K8sTarget{},
	}

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, specs, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, targ.ID(), res[targ.ID()].TargetID)

	assert.Contains(t, f.out.String(), "hello world", "expect cmd stdout in logs")
}

func TestFailedCommand(t *testing.T) {
	f := newLTFixture()
	targ := model.LocalTarget{Cmd: model.ToShellCmd("echo oh no; false")}

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty build result for failed cmd")

	require.NotNil(t, err, "failed cmd should throw error")
	assert.Contains(t, err.Error(),
		"Command \"echo oh no; false\" failed: exit status 1")
	assert.True(t, IsDontFallBackError(err), "expect DontFallBackError")

	assert.Contains(t, f.out.String(), "oh no", "expect cmd stdout in logs")
}

type ltFixture struct {
	ctx   context.Context
	out   *bytes.Buffer
	ltbad *LocalTargetBuildAndDeployer
	st    *store.Store
}

func newLTFixture() *ltFixture {
	out := new(bytes.Buffer)
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)

	ltbad := NewLocalTargetBuildAndDeployer()
	st, _ := store.NewStoreForTesting()
	return &ltFixture{
		ctx:   ctx,
		out:   out,
		ltbad: ltbad,
		st:    st,
	}
}
