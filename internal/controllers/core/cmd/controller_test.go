package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var timeout = time.Second
var interval = 5 * time.Millisecond

func TestNoop(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.step()
	f.assertCmdCount(0)
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	t2 := time.Unix(2, 0)
	f.resource("foo", "false", ".", t2)
	f.step()
	f.assertCmdDeleted("foo-serve-1")

	f.step()
	f.assertCmdMatches("foo-serve-2", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.fe.RequireNoKnownProcess(t, "true")
	f.assertLogMessage("foo", "Starting cmd false")
	f.assertLogMessage("foo", "cmd true canceled")
	f.assertCmdCount(1)
}

func TestServe(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "sleep 60", "testdir", t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil && cmd.Status.Ready
	})

	require.Equal(t, "testdir", f.fe.processes["sleep 60"].workdir)

	f.assertLogMessage("foo", "Starting cmd sleep 60")
}

func TestServeReadinessProbe(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)

	c := model.ToHostCmdInDir("sleep 60", "testdir")
	localTarget := model.NewLocalTarget("foo", model.Cmd{}, c, nil)
	localTarget.ReadinessProbe = &v1alpha1.Probe{
		TimeoutSeconds: 5,
		Handler: v1alpha1.Handler{
			Exec: &v1alpha1.ExecAction{Command: []string{"sleep", "15"}},
		},
	}

	f.resourceFromTarget("foo", localTarget, t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil && cmd.Status.Ready
	})
	f.assertLogMessage("foo", "[readiness probe: success] fake probe succeeded")

	assert.Equal(t, "sleep", f.fpm.execName)
	assert.Equal(t, []string{"15"}, f.fpm.execArgs)
	assert.GreaterOrEqual(t, f.fpm.ProbeCount(), 1)
}

func TestServeReadinessProbeInvalidSpec(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)

	c := model.ToHostCmdInDir("sleep 60", "testdir")
	localTarget := model.NewLocalTarget("foo", model.Cmd{}, c, nil)
	localTarget.ReadinessProbe = &v1alpha1.Probe{
		Handler: v1alpha1.Handler{HTTPGet: &v1alpha1.HTTPGetAction{
			// port > 65535
			Port: 70000,
		}},
	}

	f.resourceFromTarget("foo", localTarget, t1)
	f.step()

	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Terminated != nil && cmd.Status.Terminated.ExitCode == 1
	})

	f.assertLogMessage("foo", "Invalid readiness probe: port number out of range: 70000")
	assert.Equal(t, 0, f.fpm.ProbeCount())
}

func TestFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.assertLogMessage("foo", "Starting cmd true")

	err := f.fe.stop("true", 5)
	require.NoError(t, err)
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Terminated != nil && cmd.Status.Terminated.ExitCode == 5
	})

	f.assertLogMessage("foo", "cmd true exited with code 5")
}

func TestUniqueSpanIDs(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "foo.sh", ".", t1)
	f.resource("bar", "bar.sh", ".", t1)
	f.step()

	fooStart := f.waitForLogEventContaining("Starting cmd foo.sh")
	barStart := f.waitForLogEventContaining("Starting cmd bar.sh")
	require.NotEqual(t, fooStart.SpanID(), barStart.SpanID(), "different resources should have unique log span ids")
}

func TestTearDown(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "foo.sh", ".", t1)
	f.resource("bar", "bar.sh", ".", t1)
	f.step()

	f.c.TearDown(f.ctx)

	f.fe.RequireNoKnownProcess(t, "foo.sh")
	f.fe.RequireNoKnownProcess(t, "bar.sh")
}

func TestTrigger(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("cmd", "true", ".", t1)
	f.step()

	firstStart := f.assertCmdMatches("cmd-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	fw := &FileWatch{
		ObjectMeta: ObjectMeta{
			Name: "fw-1",
		},
		Spec: FileWatchSpec{
			WatchedPaths: []string{f.Path()},
		},
	}
	err := f.client.Create(f.ctx, fw)
	require.NoError(t, err)

	time.Sleep(time.Millisecond)
	f.setRestartOn("cmd-serve-1", &RestartOnSpec{
		FileWatches: []string{"fw-1"},
	})
	f.reconcileCmd("cmd-serve-1")

	secondStart := f.assertCmdMatches("cmd-serve-1", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && running.StartedAt.Time.After(firstStart.Status.Running.StartedAt.Time)
	})

	f.triggerFileWatch("fw-1")
	f.reconcileCmd("cmd-serve-1")

	f.assertCmdMatches("cmd-serve-1", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && running.StartedAt.Time.After(secondStart.Status.Running.StartedAt.Time)
	})

	// Our fixture doesn't test reconcile.Request triage,
	// so test it manually here.
	assert.Equal(f.T(),
		[]reconcile.Request{
			reconcile.Request{NamespacedName: types.NamespacedName{Name: "cmd-serve-1"}},
		},
		f.c.restartManager.enqueue(fw))
}

type testStore struct {
	*store.TestingStore
	out     io.Writer
	summary store.ChangeSummary
}

func NewTestingStore(out io.Writer) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          out,
	}
}

func (s *testStore) Cmd(name string) *Cmd {
	st := s.RLockState()
	defer s.RUnlockState()
	return st.Cmds[name]
}

func (s *testStore) CmdCount() int {
	st := s.RLockState()
	defer s.RUnlockState()
	count := 0
	for _, cmd := range st.Cmds {
		if cmd.DeletionTimestamp != nil {
			count++
		}
	}
	return count
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	st := s.LockMutableStateForTesting()
	defer s.UnlockMutableState()

	switch action := action.(type) {
	case store.ErrorAction:
		panic(fmt.Sprintf("no error action allowed: %s", action.Error))

	case store.LogAction:
		_, _ = s.out.Write(action.Message())

	case local.CmdCreateAction:
		local.HandleCmdCreateAction(st, action)
		action.Summarize(&s.summary)

	case local.CmdUpdateStatusAction:
		local.HandleCmdUpdateStatusAction(st, action)

	case local.CmdDeleteAction:
		local.HandleCmdDeleteAction(st, action)
		action.Summarize(&s.summary)
	}
}

type fixture struct {
	*tempdir.TempDirFixture
	t      *testing.T
	out    *bufsync.ThreadSafeBuffer
	st     *testStore
	fe     *FakeExecer
	fpm    *FakeProberManager
	sc     *local.ServerController
	client ctrlclient.Client
	c      *Controller
	ctx    context.Context
	cancel context.CancelFunc
}

func newFixture(t *testing.T) *fixture {
	f := tempdir.NewTempDirFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	out := bufsync.NewThreadSafeBuffer()
	w := io.MultiWriter(out, os.Stdout)
	l := logger.NewLogger(logger.VerboseLvl, w)
	ctx = logger.WithLogger(ctx, l)
	st := NewTestingStore(w)

	fe := NewFakeExecer()
	fpm := NewFakeProberManager()
	fc := fake.NewTiltClient()
	sc := local.NewServerController(fc)
	c := NewController(ctx, fe, fpm, fc, st)

	return &fixture{
		TempDirFixture: f,
		t:              t,
		st:             st,
		out:            out,
		fe:             fe,
		fpm:            fpm,
		sc:             sc,
		c:              c,
		ctx:            ctx,
		cancel:         cancel,
		client:         fc,
	}
}

func (f *fixture) teardown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}

func (f *fixture) triggerFileWatch(name string) {
	fw := &FileWatch{}
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, fw)
	require.NoError(f.T(), err)

	now := metav1.NowMicro()
	fw.Status.LastEventTime = &now
	err = f.client.Status().Update(f.ctx, fw)
	require.NoError(f.T(), err)
}

func (f *fixture) reconcileCmd(name string) {
	_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(f.T(), err)
}

func (f *fixture) setRestartOn(name string, restartOn *RestartOnSpec) {
	cmd := &Cmd{}
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, cmd)
	require.NoError(f.T(), err)

	cmd.Spec.RestartOn = restartOn
	err = f.client.Update(f.ctx, cmd)
	require.NoError(f.T(), err)
}

func (f *fixture) resource(name string, cmd string, workdir string, lastDeploy time.Time) {
	c := model.ToHostCmd(cmd)
	c.Dir = workdir
	localTarget := model.NewLocalTarget(model.TargetName(name), model.Cmd{}, c, nil)
	f.resourceFromTarget(name, localTarget, lastDeploy)
}

func (f *fixture) resourceFromTarget(name string, target model.TargetSpec, lastDeploy time.Time) {
	n := model.ManifestName(name)
	m := model.Manifest{
		Name: n,
	}.WithDeployTarget(target)

	st := f.st.LockMutableStateForTesting()
	defer f.st.UnlockMutableState()
	st.UpsertManifestTarget(&store.ManifestTarget{
		Manifest: m,
		State: &store.ManifestState{
			LastSuccessfulDeployTime: lastDeploy,
		},
	})
}

func (f *fixture) step() {
	f.st.summary = store.ChangeSummary{}
	f.sc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	for name := range f.st.summary.CmdSpecs {
		_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
		require.NoError(f.t, err)
	}
}

func (f *fixture) assertLogMessage(name string, messages ...string) {
	for _, m := range messages {
		assert.Eventually(f.t, func() bool {
			return strings.Contains(f.out.String(), m)
		}, timeout, interval)
	}
}

func (f *fixture) waitForLogEventContaining(message string) store.LogAction {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	for {
		actions := f.st.Actions()
		for _, action := range actions {
			le, ok := action.(store.LogAction)
			if ok && strings.Contains(string(le.Message()), message) {
				return le
			}
		}
		select {
		case <-ctx.Done():
			f.t.Fatalf("timed out waiting for log event w/ message %q. seen actions: %v", message, actions)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (f *fixture) assertCmdMatches(name string, matcher func(cmd *Cmd) bool) *Cmd {
	assert.Eventually(f.t, func() bool {
		cmd := f.st.Cmd(name)
		if cmd == nil {
			return false
		}
		return matcher(cmd)
	}, timeout, interval)

	var cmd Cmd
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, &cmd)
	require.NoError(f.t, err)
	assert.True(f.t, matcher(&cmd))
	return &cmd
}

func (f *fixture) assertCmdDeleted(name string) {
	assert.Eventually(f.t, func() bool {
		cmd := f.st.Cmd(name)
		return cmd == nil || cmd.DeletionTimestamp != nil
	}, timeout, interval)

	var cmd Cmd
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, &cmd)
	assert.Error(f.t, err)
	assert.True(f.t, apierrors.IsNotFound(err))
}

func (f *fixture) assertCmdCount(count int) {
	assert.Equal(f.t, count, f.st.CmdCount())

	var list CmdList
	err := f.client.List(f.ctx, &list)
	require.NoError(f.t, err)
	assert.Equal(f.t, count, len(list.Items))
}
