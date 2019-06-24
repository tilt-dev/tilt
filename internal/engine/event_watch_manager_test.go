package engine

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestEventWatchManager_dispatchesEvent(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	f.addManifest("someK8sManifest")

	evt := &v1.Event{
		Reason:  "because test",
		Message: "hello world",
	}

	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	expected := store.K8sEventAction{Event: evt}
	f.assertActions(expected)
}

func TestEventWatchManager_watchError(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	err := fmt.Errorf("oh noes")
	f.kClient.EventsWatchErr = err
	f.addManifest("someK8sManifest")

	f.ewm.OnChange(f.ctx, f.store)

	expectedErr := errors.Wrap(err, "Error watching k8s events\n")
	expected := store.ErrorAction{Error: expectedErr}
	f.assertActions(expected)
}

func TestEventWatchManager_needsWatchNoK8s(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	evt := &v1.Event{
		Reason:  "because test",
		Message: "hello world",
	}

	// No k8s manifests on the state, so OnChange shouldn't do anything --
	// when we emit an event, we do NOT expect to see an action dispatched,
	// since no watch should have been started.
	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	f.assertNoActions()
}

type ewmFixture struct {
	t          *testing.T
	kClient    *k8s.FakeK8sClient
	ewm        *EventWatchManager
	ctx        context.Context
	cancel     func()
	store      *store.Store
	getActions func() []store.Action

	// old value of k8sEventsFeatureFlag env var, for teardown
	// TODO(maia): remove this when we remove the feature flag
	oldFeatureFlagVal string
}

func newEWMFixture(t *testing.T) *ewmFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx := output.CtxForTest()
	ctx, cancel := context.WithCancel(ctx)

	ret := &ewmFixture{
		kClient:           kClient,
		ewm:               NewEventWatchManager(kClient),
		ctx:               ctx,
		cancel:            cancel,
		t:                 t,
		oldFeatureFlagVal: os.Getenv(k8sEventsFeatureFlag),
	}

	os.Setenv(k8sEventsFeatureFlag, "true")

	ret.store, ret.getActions = store.NewStoreForTesting()
	go ret.store.Loop(ctx)

	return ret
}

func (f *ewmFixture) TearDown() {
	f.kClient.TearDown()

	_ = os.Setenv(k8sEventsFeatureFlag, f.oldFeatureFlagVal)
	f.cancel()
}

func (f *ewmFixture) addManifest(manifestName string) {
	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true
	dt := model.K8sTarget{Name: model.TargetName(manifestName)}
	m := model.Manifest{Name: model.ManifestName(manifestName)}.WithDeployTarget(dt)
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()
}

func (f *ewmFixture) assertNoActions() {
	f.assertActions()
}

func (f *ewmFixture) assertActions(expected ...store.Action) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) >= len(expected) {
			break
		}
	}

	// Make extra sure we didn't get any extra actions
	time.Sleep(10 * time.Millisecond)

	// NOTE(maia): this test will break if this the code ever returns other
	// correct-but-incidental-to-this-test actions, but for now it's fine.
	actual := f.getActions()
	if !assert.Len(f.t, actual, len(expected)) {
		f.t.FailNow()
	}

	for i, a := range actual {
		switch exp := expected[i].(type) {
		case store.ErrorAction:
			// Special case -- we can't just assert.Equal b/c pointer equality stuff
			act, ok := a.(store.ErrorAction)
			if !ok {
				f.t.Fatalf("got non-%T: %v", store.ErrorAction{}, a)
			}
			assert.Equal(f.t, exp.Error.Error(), act.Error.Error())
		default:
			assert.Equal(f.t, expected[i], a)
		}
	}
}
