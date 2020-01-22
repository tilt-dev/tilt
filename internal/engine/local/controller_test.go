package local

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/bufsync"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
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
	f.resource("foo", "true", t1)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 1)

	t2 := time.Unix(2, 0)
	f.resource("foo", "false", t2)
	f.step()
	f.assertStatus("foo", model.RuntimeStatusOK, 2)
	f.assertNoAction("error for cancel", func(action store.Action) bool {
		a, ok := action.(LocalServeStatusAction)
		if !ok {
			return false
		}
		return a.ManifestName == "foo" && a.Status == model.RuntimeStatusError
	})
	f.assertNoAction("log for cancel", func(action store.Action) bool {
		a, ok := action.(store.LogAction)
		if !ok {
			return false
		}
		return a.ManifestName() == "foo" && strings.Contains(string(a.Message()), "cmd true canceled")
	})
	f.fe.RequireNoKnownProcess(t, "true")
	f.assertLogMessage("foo", "Starting cmd false")
}

func TestFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", t1)
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
	f.resource("foo", "foo.sh", t1)
	f.resource("bar", "bar.sh", t1)
	f.step()

	fooStart := f.waitForLogEventContaining("Starting cmd foo.sh")
	barStart := f.waitForLogEventContaining("Starting cmd bar.sh")
	require.NotEqual(t, fooStart.SpanID(), barStart.SpanID(), "different resources should have unique log span ids")
}

func TestTearDown(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "foo.sh", t1)
	f.resource("bar", "bar.sh", t1)
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
	c      *Controller
	ctx    context.Context
	cancel context.CancelFunc
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	out := bufsync.NewThreadSafeBuffer()
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)

	fe := NewFakeExecer()

	return &fixture{
		t:  t,
		st: store.NewTestingStore(),
		state: store.EngineState{
			ManifestTargets: make(map[model.ManifestName]*store.ManifestTarget),
		},
		fe:     fe,
		c:      NewController(fe),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (f *fixture) teardown() {
	f.cancel()
}

func (f *fixture) resource(name string, cmd string, lastDeploy time.Time) {
	n := model.ManifestName(name)
	m := model.Manifest{
		Name: n,
	}.WithDeployTarget(model.NewLocalTarget(
		model.TargetName(name), model.Cmd{}, model.ToShellCmd(cmd), nil, ""))
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
