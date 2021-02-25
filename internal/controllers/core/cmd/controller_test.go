package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/bufsync"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var timeout = time.Second
var interval = 5 * time.Millisecond

func TestUpdate(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cmdTrue := f.cmd("foo", "true", ".")
	f.create(cmdTrue)
	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Running != nil
	})

	cmdFalse := f.cmd("foo", "false", ".")
	f.update(cmdFalse)
	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Running != nil && cmp.Equal(cmdFalse.Spec, cmd.Spec)
	})

	f.assertNoProcessExists(cmdTrue)

	f.assertLogMessage("foo", "Starting cmd false")
	f.assertLogMessage("foo", "cmd true canceled")
}

func TestServe(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cmd := f.cmd("foo", "sleep 60", "testdir")
	f.create(cmd)
	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Running != nil && cmd.Status.Ready
	})

	require.Equal(t, "testdir", f.fe.getProcess(cmd).workdir)

	f.assertLogMessage("foo", "Starting cmd sleep 60")
}

func TestServeReadinessProbe(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cmd := f.cmd("foo", "sleep 60", "testdir")
	cmd.Spec.ReadinessProbe = &v1alpha1.Probe{
		TimeoutSeconds: 5,
		Handler: v1alpha1.Handler{
			Exec: &v1alpha1.ExecAction{Command: []string{"sleep", "15"}},
		},
	}

	f.create(cmd)
	f.assertCmdMatches("foo", func(cmd Cmd) bool {
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

	cmd := f.cmd("foo", "sleep 60", "testdir")
	cmd.Spec.ReadinessProbe = &v1alpha1.Probe{
		Handler: v1alpha1.Handler{HTTPGet: &v1alpha1.HTTPGetAction{
			// port > 65535
			Port: 70000,
		}},
	}
	f.create(cmd)

	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Terminated != nil && cmd.Status.Terminated.ExitCode == 1
	})

	f.assertLogMessage("foo", "Invalid readiness probe: port number out of range: 70000")
	assert.Equal(t, 0, f.fpm.ProbeCount())
}

func TestFailure(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cmd := f.cmd("foo", "true", ".")
	f.create(cmd)
	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Running != nil
	})
	f.assertLogMessage("foo", "Starting cmd true")

	err := f.fe.stop("true", 5)
	require.NoError(t, err)

	f.assertCmdMatches("foo", func(cmd Cmd) bool {
		return cmd.Status.Terminated != nil && cmd.Status.Terminated.ExitCode != 5
	})
	f.assertLogMessage("foo", "cmd true exited with code 5")
}

func TestUniqueSpanIDs(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	f.create(f.cmd("foo", "foo.sh", "."))
	f.create(f.cmd("bar", "bar.sh", "."))

	fooStart := f.waitForLogEventContaining("Starting cmd foo.sh")
	barStart := f.waitForLogEventContaining("Starting cmd bar.sh")
	require.NotEqual(t, fooStart.SpanID(), barStart.SpanID(), "different resources should have unique log span ids")
}

func TestTearDown(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	cmdFoo := f.cmd("foo", "foo.sh", ".")
	f.create(cmdFoo)

	cmdBar := f.cmd("bar", "bar.sh", ".")
	f.create(cmdBar)

	f.c.TearDown(f.ctx)

	f.assertNoProcessExists(cmdFoo)
	f.assertNoProcessExists(cmdBar)
}

type testStore struct {
	*store.TestingStore
	ctx context.Context
}

func NewTestingStore(ctx context.Context) *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		ctx:          ctx,
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	logAction, ok := action.(store.LogAction)
	if ok {
		logger.Get(s.ctx).Infof("%s", logAction.Message())
	}
}

type fixture struct {
	t      *testing.T
	st     *testStore
	fe     *FakeExecer
	fpm    *FakeProberManager
	c      *Controller
	ctx    context.Context
	cancel context.CancelFunc
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	out := bufsync.NewThreadSafeBuffer()
	l := logger.NewLogger(logger.VerboseLvl, out)
	ctx = logger.WithLogger(ctx, l)
	st := NewTestingStore(ctx)

	scheme := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(scheme)
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	fe := NewFakeExecer()
	fpm := NewFakeProberManager()
	controller := NewController(st, fe, fpm)
	controller.SetClient(client)

	return &fixture{
		t:      t,
		st:     st,
		fe:     fe,
		fpm:    fpm,
		c:      controller,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (f *fixture) teardown() {
	f.cancel()
	f.c.TearDown(context.Background())
}

func (f *fixture) create(r *Cmd) {
	require.NoError(f.t, f.c.Client.Create(f.ctx, r))

	key := types.NamespacedName{Name: r.Name}
	_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: key})
	require.NoError(f.t, err)
}

func (f *fixture) update(r *Cmd) {
	var old Cmd
	key := types.NamespacedName{Name: r.Name}
	require.NoError(f.t, f.c.Client.Get(f.ctx, key, &old))

	r.ObjectMeta.ResourceVersion = old.ResourceVersion

	require.NoError(f.t, f.c.Client.Update(f.ctx, r))

	_, err := f.c.Reconcile(f.ctx, ctrl.Request{NamespacedName: key})
	require.NoError(f.t, err)
}

func (f *fixture) cmd(name string, cmd string, workdir string) *Cmd {
	return &Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				v1alpha1.LabelManifest: name,
			},
		},
		Spec: CmdSpec{
			Args: model.ToHostCmd(cmd).Argv,
			Dir:  workdir,
		},
	}
}

func (f *fixture) assertNoProcessExists(cmd *Cmd) {
	assert.Eventually(f.t, func() bool {
		return f.fe.getProcess(cmd) == nil
	}, timeout, interval)
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

func (f *fixture) assertCmdMatches(name string, matcher func(cmd Cmd) bool) {
	key := types.NamespacedName{Name: name}
	var cmd Cmd
	assert.Eventually(f.t, func() bool {
		err := f.c.Get(f.ctx, key, &cmd)
		if err != nil {
			return false
		}
		return matcher(cmd)
	}, timeout, interval)
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
