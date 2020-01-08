package local

import (
	"context"
	"testing"
	"time"

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

func TestSimple(t *testing.T) {
	f := newFixture(t)
	defer f.teardown()

	t1 := time.Unix(1, 0)
	f.resource("foo", "true", t1)
	f.step()
	f.assertStatus("foo", Running, 1)

	t2 := time.Unix(2, 0)
	f.resource("foo", "", t2)
	f.step()
	f.assertStatus("foo", Done, 1)
}

type fixture struct {
	t      *testing.T
	st     *store.TestingStore
	state  store.EngineState
	c      *Controller
	ctx    context.Context
	cancel context.CancelFunc
	locals []string
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	out := bufsync.NewThreadSafeBuffer()
	l := logger.NewLogger(logger.DebugLvl, out)
	ctx = logger.WithLogger(ctx, l)

	return &fixture{
		t:  t,
		st: store.NewTestingStore(),
		state: store.EngineState{
			ManifestTargets: make(map[model.ManifestName]*store.ManifestTarget),
		},
		c:      NewController(NewFakeExecer()),
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
	time.Sleep(10 * time.Millisecond)
}

func (f *fixture) assertNoStatus() {
	actions := f.st.Actions()
	if len(actions) > 0 {
		f.t.Fatalf("expected no actions")
	}
}

func (f *fixture) assertStatus(name string, status Status, sequenceNum int) {
	actions := f.st.Actions()
	for _, action := range actions {
		stAction, ok := action.(LocalServeStatusAction)
		if !ok ||
			stAction.ManifestName != model.ManifestName(name) ||
			stAction.Status != status ||
			stAction.SequenceNum != sequenceNum {
			continue
		}
		return
	}

	f.t.Fatalf("didn't find name %s, status %v, sequence %d in %+v", name, status, sequenceNum, actions)
}
