package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestNoop(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.step()
	f.assertNoStatus()
}

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 1)

	t2 := time.Unix(2, 0)
	f.resource("foo", "false", ".", t2)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 2)
	f.assertNoAction("error for cancel", func(action store.Action) bool {
		a, ok := action.(LocalServeStatusAction)
		if !ok {
			return false
		}
		return a.ManifestName == "foo" && a.Status == model.RuntimeStatusError
	})
	f.fe.RequireNoKnownProcess(t, "true")
	f.assertLogMessage("foo", "Starting cmd false")
	f.assertLogMessage("foo", "cmd true canceled")
}

func TestServe(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "sleep 60", "testdir", t1)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 1)

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
	f.assertAction("did not find readiness probe action", func(action store.Action) bool {
		probeAction, ok := action.(LocalServeReadinessProbeAction)
		if !ok ||
			probeAction.ManifestName != "foo" ||
			probeAction.Ready != true {
			return false
		}
		return true
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

	f.assertStatus("foo", model.RuntimeStatusError, 1)

	f.assertLogMessage("foo", "Invalid readiness probe: port number out of range: 70000")
	assert.Equal(t, 0, f.fpm.ProbeCount())
}

func TestFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", ".", t1)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 1)
	f.assertLogMessage("foo", "Starting cmd true")

	err := f.fe.stop("true", 5)
	require.NoError(t, err)

	f.assertStatus("foo", model.RuntimeStatusError, 1)
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

type fixture struct {
	t      *testing.T
	st     *store.TestingStore
	state  store.EngineState
	fe     *FakeExecer
	fpm    *FakeProberManager
	c      *ControllerOld
	ctx    context.Context
	cancel context.CancelFunc
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	out := bufsync.NewThreadSafeBuffer()
	l := logger.NewLogger(logger.VerboseLvl, out)
	ctx = logger.WithLogger(ctx, l)

	fe := NewFakeExecer()
	fpm := NewFakeProberManager()

	return &fixture{
		t:  t,
		st: store.NewTestingStore(),
		state: store.EngineState{
			ManifestTargets: make(map[model.ManifestName]*store.ManifestTarget),
		},
		fe:     fe,
		fpm:    fpm,
		c:      NewControllerOld(fe, fpm),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (f *fixture) teardown() {
	f.cancel()
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
	f.state.UpsertManifestTarget(&store.ManifestTarget{
		Manifest: m,
		State: &store.ManifestState{
			LastSuccessfulDeployTime: lastDeploy,
		},
	})
}

func (f *fixture) step() {
	f.st.ClearActions()
	f.st.SetState(f.state)
	f.c.OnChange(f.ctx, f.st)
}

func (f *fixture) assertNoStatus() {
	actions := f.st.Actions()
	if len(actions) > 0 {
		f.t.Fatalf("expected no actions")
	}
}

func (f *fixture) assertAction(msg string, pred func(action store.Action) bool) {
	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	for {
		actions := f.st.Actions()
		for _, action := range actions {
			if pred(action) {
				return
			}
		}
		select {
		case <-ctx.Done():
			f.t.Fatalf("%s. seen actions: %v", msg, actions)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

func (f *fixture) assertNoAction(msg string, pred func(action store.Action) bool) {
	for _, action := range f.st.Actions() {
		require.Falsef(f.t, pred(action), "%s", msg)
	}
}

func (f *fixture) assertStatus(name string, status model.RuntimeStatus, sequenceNum int) {
	msg := fmt.Sprintf("didn't find name %s, status %v, sequence %d", name, status, sequenceNum)
	pred := func(action store.Action) bool {
		stAction, ok := action.(LocalServeStatusAction)
		if !ok ||
			stAction.ManifestName != model.ManifestName(name) ||
			stAction.Status != status {
			return false
		}
		return true
	}

	f.assertAction(msg, pred)
}

func (f *fixture) assertLogMessage(name string, messages ...string) {
	for _, m := range messages {
		msg := fmt.Sprintf("didn't find name %s, message %s", name, m)
		pred := func(action store.Action) bool {
			a, ok := action.(store.LogAction)
			return ok && strings.Contains(string(a.Message()), m)
		}
		f.assertAction(msg, pred)
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
