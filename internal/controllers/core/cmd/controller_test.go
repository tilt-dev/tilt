package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
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
	"github.com/tilt-dev/tilt/pkg/apis"
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

func TestUpdateWithCurrentBuild(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.st.WithState(func(s *store.EngineState) {
		c := model.ToHostCmd("false")
		localTarget := model.NewLocalTarget(model.TargetName("foo"), c, c, nil)
		s.ManifestTargets["foo"].Manifest.DeployTarget = localTarget
		s.ManifestTargets["foo"].State.CurrentBuild = model.BuildRecord{StartTime: f.clock.Now()}
	})

	f.step()

	assert.Never(f.t, func() bool {
		return f.st.Cmd("foo-serve-2") != nil
	}, 20*time.Millisecond, 5*time.Millisecond)

	f.st.WithState(func(s *store.EngineState) {
		s.ManifestTargets["foo"].State.CurrentBuild = model.BuildRecord{}
	})

	f.step()
	f.assertCmdDeleted("foo-serve-1")
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

func TestRestartOnFileWatch(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.resource("cmd", "true", ".", f.clock.Now())
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

	f.clock.Advance(time.Second)
	f.updateSpec("cmd-serve-1", func(spec *v1alpha1.CmdSpec) {
		spec.RestartOn = &RestartOnSpec{
			FileWatches: []string{"fw-1"},
		}
	})

	f.clock.Advance(time.Second)
	f.triggerFileWatch("fw-1")
	f.reconcileCmd("cmd-serve-1")

	f.assertCmdMatches("cmd-serve-1", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && running.StartedAt.Time.After(firstStart.Status.Running.StartedAt.Time)
	})

	// Our fixture doesn't test reconcile.Request triage,
	// so test it manually here.
	assert.Equal(f.T(),
		[]reconcile.Request{
			reconcile.Request{NamespacedName: types.NamespacedName{Name: "cmd-serve-1"}},
		},
		f.c.indexer.Enqueue(fw))
}

func setupStartOnTest(t *testing.T, f *fixture) {
	cmd := &Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcmd",
		},
		Spec: v1alpha1.CmdSpec{
			Args: []string{"myserver"},
			StartOn: &StartOnSpec{
				UIButtons:  []string{"b-1"},
				StartAfter: apis.NewTime(f.clock.Now()),
			},
		},
	}

	err := f.client.Create(f.ctx, cmd)
	require.NoError(t, err)

	b := &UIButton{
		ObjectMeta: ObjectMeta{
			Name: "b-1",
		},
		Spec: UIButtonSpec{},
	}
	err = f.client.Create(f.ctx, b)
	require.NoError(t, err)

	f.reconcileCmd("testcmd")

	f.fe.RequireNoKnownProcess(t, "myserver")
}

func TestStartOnNoPreviousProcess(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	startup := f.clock.Now()

	setupStartOnTest(t, f)

	f.clock.Advance(time.Second)

	f.triggerButton("b-1", f.clock.Now())
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && running.StartedAt.Time.After(startup)
	})
}

func TestStartOnDoesntRunOnCreation(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	setupStartOnTest(t, f)

	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason
	})

	f.fe.RequireNoKnownProcess(t, "myserver")
}

func TestStartOnStartAfter(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	setupStartOnTest(t, f)

	f.triggerButton("b-1", f.clock.Now().Add(-time.Minute))

	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason
	})

	f.fe.RequireNoKnownProcess(t, "myserver")
}

func TestStartOnRunningProcess(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	setupStartOnTest(t, f)

	f.clock.Advance(time.Second)
	f.triggerButton("b-1", f.clock.Now())
	f.reconcileCmd("testcmd")

	// wait for the initial process to start
	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.fe.mu.Lock()
	st := f.fe.processes["myserver"].startTime
	f.fe.mu.Unlock()

	f.clock.Advance(time.Second)

	secondClickTime := f.clock.Now()
	f.triggerButton("b-1", secondClickTime)
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && !running.StartedAt.Time.Before(secondClickTime)
	})

	// make sure it's not the same process
	f.fe.mu.Lock()
	p, ok := f.fe.processes["myserver"]
	require.True(t, ok)
	require.NotEqual(t, st, p.startTime)
	f.fe.mu.Unlock()
}

func TestStartOnPreviousTerminatedProcess(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	firstClickTime := f.clock.Now()

	setupStartOnTest(t, f)

	f.triggerButton("b-1", firstClickTime)
	f.reconcileCmd("testcmd")

	// wait for the initial process to start
	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.fe.mu.Lock()
	st := f.fe.processes["myserver"].startTime
	f.fe.mu.Unlock()

	err := f.fe.stop("myserver", 1)
	require.NoError(t, err)

	// wait for the initial process to die
	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Terminated != nil
	})

	f.clock.Advance(time.Second)
	secondClickTime := f.clock.Now()
	f.triggerButton("b-1", secondClickTime)
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && !running.StartedAt.Time.Before(secondClickTime)
	})

	// make sure it's not the same process
	f.fe.mu.Lock()
	p, ok := f.fe.processes["myserver"]
	require.True(t, ok)
	require.NotEqual(t, st, p.startTime)
	f.fe.mu.Unlock()
}

func TestDisposeOrphans(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.st.WithState(func(es *store.EngineState) {
		es.RemoveManifestTarget("foo")
	})
	f.step()
	f.assertCmdCount(0)
	f.fe.RequireNoKnownProcess(t, "true")
}

func TestDisposeTerminatedWhenCmdChanges(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.MkdirAll("subdir")

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()

	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	err := f.fe.stop("true", 0)
	require.NoError(t, err)

	f.assertCmdMatches("foo-serve-1", func(cmd *Cmd) bool {
		return cmd.Status.Terminated != nil
	})

	f.resource("foo", "true", "subdir", t1)
	f.step()
	f.assertCmdMatches("foo-serve-2", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})
	f.assertCmdDeleted("foo-serve-1")
}

// Self-modifying Cmds are typically paired with a StartOn trigger,
// to simulate a "toggle" switch on the Cmd.
//
// See:
// https://github.com/tilt-dev/tilt-extensions/issues/202
func TestSelfModifyingCmd(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	setupStartOnTest(t, f)

	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason
	})

	f.clock.Advance(time.Second)
	f.triggerButton("b-1", f.clock.Now())
	f.clock.Advance(time.Second)
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	f.updateSpec("testcmd", func(spec *v1alpha1.CmdSpec) {
		spec.Args = []string{"yourserver"}
	})
	f.reconcileCmd("testcmd")
	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason
	})

	f.fe.RequireNoKnownProcess(t, "myserver")
	f.fe.RequireNoKnownProcess(t, "yourserver")
	f.clock.Advance(time.Second)
	f.triggerButton("b-1", f.clock.Now())
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})
}

// Ensure that changes to the StartOn or RestartOn fields
// don't restart the command.
func TestDependencyChangesDoNotCauseRestart(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	setupStartOnTest(t, f)
	f.triggerButton("b-1", f.clock.Now())
	f.clock.Advance(time.Second)
	f.reconcileCmd("testcmd")

	firstStart := f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		return cmd.Status.Running != nil
	})

	err := f.client.Create(f.ctx, &v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "new-button"}})
	require.NoError(t, err)

	err = f.client.Create(f.ctx, &v1alpha1.FileWatch{
		ObjectMeta: metav1.ObjectMeta{Name: "new-filewatch"},
		Spec: FileWatchSpec{
			WatchedPaths: []string{f.JoinPath("new-path")},
		},
	})
	require.NoError(t, err)

	f.updateSpec("testcmd", func(spec *v1alpha1.CmdSpec) {
		spec.StartOn = &v1alpha1.StartOnSpec{
			UIButtons: []string{"new-button"},
		}
		spec.RestartOn = &v1alpha1.RestartOnSpec{
			FileWatches: []string{"new-filewatch"},
		}
	})
	f.reconcileCmd("testcmd")

	f.assertCmdMatchesInAPI("testcmd", func(cmd *Cmd) bool {
		running := cmd.Status.Running
		return running != nil && running.StartedAt.Time.Equal(firstStart.Status.Running.StartedAt.Time)
	})
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
		if cmd.DeletionTimestamp == nil {
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
	clock  clockwork.FakeClock
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
	fc := fake.NewFakeTiltClient()
	sc := local.NewServerController(fc)
	clock := clockwork.NewFakeClock()
	c := NewController(ctx, fe, fpm, fc, st, clock, v1alpha1.NewScheme())

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
		clock:          clock,
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

	fw.Status.LastEventTime = apis.NewMicroTime(f.clock.Now())
	err = f.client.Status().Update(f.ctx, fw)
	require.NoError(f.T(), err)
}

func (f *fixture) triggerButton(name string, ts time.Time) {
	b := &UIButton{}
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, b)
	require.NoError(f.T(), err)

	b.Status.LastClickedAt = metav1.NewMicroTime(ts)
	err = f.client.Status().Update(f.ctx, b)
	require.NoError(f.T(), err)
}

func (f *fixture) reconcileCmd(name string) {
	_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name}})
	require.NoError(f.T(), err)
}

func (f *fixture) updateSpec(name string, update func(spec *v1alpha1.CmdSpec)) {
	cmd := &Cmd{}
	err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, cmd)
	require.NoError(f.T(), err)

	update(&(cmd.Spec))
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

	state := &store.ManifestState{
		LastSuccessfulDeployTime: lastDeploy,
	}
	state.AddCompletedBuild(model.BuildRecord{
		StartTime:  lastDeploy,
		FinishTime: lastDeploy,
	})
	st.UpsertManifestTarget(&store.ManifestTarget{
		Manifest: m,
		State:    state,
	})
}

func (f *fixture) step() {
	f.st.summary = store.ChangeSummary{}
	_ = f.sc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	for name := range f.st.summary.CmdSpecs.Changes {
		_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: name})
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

	return f.assertCmdMatchesInAPI(name, matcher)
}

func (f *fixture) assertCmdMatchesInAPI(name string, matcher func(cmd *Cmd) bool) *Cmd {
	f.t.Helper()
	var cmd Cmd

	assert.Eventually(f.t, func() bool {
		err := f.client.Get(f.ctx, types.NamespacedName{Name: name}, &cmd)
		require.NoError(f.t, err)
		return matcher(&cmd)
	}, timeout, interval)

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
