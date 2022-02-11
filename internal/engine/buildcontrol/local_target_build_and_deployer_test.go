package buildcontrol

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/core/cmd"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
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

	f.MkdirAll(filepath.Join("some", "internal", "dir"))
	workdir := f.JoinPath("some", "internal", "dir")
	cmd := "echo the directory is $(pwd)"
	if runtime.GOOS == "windows" {
		cmd = "echo the directory is %cd%"
	}
	targ := f.localTargetWithWorkdir(cmd, workdir)

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

	targ := f.localTarget("echo oh no && exit 1")

	res, err := f.ltbad.BuildAndDeploy(f.ctx, f.st, []model.TargetSpec{targ}, store.BuildStateSet{})
	assert.Empty(t, res, "expect empty build result for failed cmd")

	require.NotNil(t, err, "failed cmd should throw error")
	assert.Contains(t, err.Error(),
		"Command \"echo oh no && exit 1\" failed: exit status 1")
	assert.True(t, IsDontFallBackError(err), "expect DontFallBackError")

	assert.Contains(t, f.out.String(), "oh no", "expect cmd stdout in logs")
}

type testStore struct {
	*store.TestingStore
	out io.Writer
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	if action, ok := action.(store.LogAction); ok {
		_, _ = s.out.Write(action.Message())
	}
}

type ltFixture struct {
	*tempdir.TempDirFixture

	ctx        context.Context
	out        *bytes.Buffer
	ltbad      *LocalTargetBuildAndDeployer
	st         *testStore
	ctrlClient ctrlclient.Client
}

func newLTFixture(t *testing.T) *ltFixture {
	f := tempdir.NewTempDirFixture(t)

	out := new(bytes.Buffer)
	ctx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)
	clock := fakeClock{time.Date(2019, 1, 1, 1, 1, 1, 1, time.UTC)}

	ctrlClient := fake.NewFakeTiltClient()

	fe := cmd.NewProcessExecer(localexec.EmptyEnv())
	fpm := cmd.NewFakeProberManager()
	cclock := clockwork.NewFakeClock()
	st := NewTestingStore(out)
	cmds := cmd.NewController(ctx, fe, fpm, ctrlClient, st, cclock, v1alpha1.NewScheme())
	ltbad := NewLocalTargetBuildAndDeployer(clock, ctrlClient, cmds)

	return &ltFixture{
		TempDirFixture: f,
		ctx:            ctx,
		out:            out,
		ltbad:          ltbad,
		st:             st,
		ctrlClient:     ctrlClient,
	}
}

func (f *ltFixture) localTarget(cmd string) model.LocalTarget {
	return f.localTargetWithWorkdir(cmd, f.Path())
}

func (f *ltFixture) localTargetWithWorkdir(cmd string, workdir string) model.LocalTarget {
	c := model.ToHostCmd(cmd)
	c.Dir = workdir
	lt := model.NewLocalTarget("local", c, model.Cmd{}, nil)

	cmdObj := &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{Name: lt.UpdateCmdName()},
		Spec:       *(lt.UpdateCmdSpec),
	}
	require.NoError(f.T(), f.ctrlClient.Create(f.ctx, cmdObj))
	return lt
}
