package engine

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestNoLocalTargets(t *testing.T) {
	f := newLTFixture(t)
	defer f.TearDown()

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
	f := newLTFixture(t)
	defer f.TearDown()

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
	f := newLTFixture(t)
	defer f.TearDown()

	targ := f.localTarget("echo hello world")

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, targ.ID(), res[targ.ID()].TargetID())

	assert.Contains(t, f.out.String(), "hello world", "expect cmd stdout in logs")
}

func TestWorkdir(t *testing.T) {
	f := newLTFixture(t)
	defer f.TearDown()

	f.MkdirAll("some/internal/dir")
	workdir := f.JoinPath("some/internal/dir")
	targ := f.localTargetWithWorkdir("echo the directory is $(pwd)", workdir)

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, targ.ID(), res[targ.ID()].TargetID())

	expectedOut := fmt.Sprintf("the directory is %s", workdir)
	assert.Contains(t, f.out.String(), expectedOut, "expect cmd stdout (with appropriate pwd) in logs")
}

func TestExtractOneLocalTarget(t *testing.T) {
	f := newLTFixture(t)
	defer f.TearDown()

	targ := f.localTarget("echo hello world")

	// Even if there are multiple other targets, should correctly extract and run the one LocalTarget
	specs := []model.TargetSpec{
		targ, model.ImageTarget{}, model.K8sTarget{},
	}

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, specs, store.BuildStateSet{})
	require.Nil(t, err)

	assert.Equal(t, targ.ID(), res[targ.ID()].TargetID())

	assert.Contains(t, f.out.String(), "hello world", "expect cmd stdout in logs")
}

func TestFailedCommand(t *testing.T) {
	f := newLTFixture(t)
	defer f.TearDown()

	targ := f.localTarget("echo oh no; false")

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty build result for failed cmd")

	require.NotNil(t, err, "failed cmd should throw error")
	assert.Contains(t, err.Error(),
		"Command \"echo oh no; false\" failed: exit status 1")
	assert.True(t, buildcontrol.IsDontFallBackError(err), "expect DontFallBackError")

	assert.Contains(t, f.out.String(), "oh no", "expect cmd stdout in logs")
}

type ltFixture struct {
	*tempdir.TempDirFixture

	ctx   context.Context
	out   *bytes.Buffer
	ltbad *LocalTargetBuildAndDeployer
	st    *store.Store
}

func newLTFixture(t *testing.T) *ltFixture {
	f := tempdir.NewTempDirFixture(t)

	out := new(bytes.Buffer)
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)
	clock := fakeClock{time.Date(2019, 1, 1, 1, 1, 1, 1, time.UTC)}

	ltbad := NewLocalTargetBuildAndDeployer(clock)
	st, _ := store.NewStoreForTesting()
	return &ltFixture{
		TempDirFixture: f,
		ctx:            ctx,
		out:            out,
		ltbad:          ltbad,
		st:             st,
	}
}

func (f *ltFixture) localTarget(cmd string) model.LocalTarget {
	return f.localTargetWithWorkdir(cmd, f.Path())
}

func (f *ltFixture) localTargetWithWorkdir(cmd string, workdir string) model.LocalTarget {
	return model.LocalTarget{
		UpdateCmd: model.ToShellCmd(cmd),
		Workdir:   workdir,
	}
}
