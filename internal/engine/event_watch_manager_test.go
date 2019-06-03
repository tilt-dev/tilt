package engine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
	v1 "k8s.io/api/core/v1"
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
	expectedAction := store.K8SEventAction{Event: evt}
	f.assertObservedK8sEventActions(expectedAction)

}
func TestEventWatchManager_needsWatchNoK8s(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	evt := &v1.Event{
		Reason:  "because test",
		Message: "hello world",
	}

	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	f.assertNoK8sEventActions()
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

func (f *ewmFixture) assertNoK8sEventActions() {
	f.assertObservedK8sEventActions()
}

func (f *ewmFixture) assertObservedK8sEventActions(expectedActions ...store.K8SEventAction) {
	if len(expectedActions) == 0 {
		// assert no k8s event actions -- sleep briefly
		// to give any actions a chance to get into the queue
		time.Sleep(10 * time.Millisecond)
	}

	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) == len(expectedActions) {
			break
		}
	}

	var observedActions []store.K8SEventAction
	for _, a := range f.getActions() {
		sca, ok := a.(store.K8SEventAction)
		if !ok {
			f.t.Fatalf("got non-%T: %v", store.K8SEventAction{}, a)
		}
		observedActions = append(observedActions, sca)
	}

	assert.Equal(f.t, expectedActions, observedActions)
}
