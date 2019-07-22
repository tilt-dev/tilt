package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestEventWatchManager_dispatchesEvent(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	f.addManifest(mn)
	obj := f.makeObj(mn)
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{Name: obj.GetName()}: &obj,
	}

	evt := f.makeEvent(obj)

	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	gvk := obj.GroupVersionKind()
	e := k8s.K8sEntity{
		Obj:  &obj,
		Kind: &gvk,
	}
	expected := store.K8sEventAction{Event: evt, ManifestName: mn, InvolvedObject: e}
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

	mn := model.ManifestName("someK8sManifest")

	obj := f.makeObj(mn)
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{Name: obj.GetName()}: &obj,
	}

	evt := f.makeEvent(obj)

	// No k8s manifests on the state, so OnChange shouldn't do anything --
	// when we emit an event, we do NOT expect to see an action dispatched,
	// since no watch should have been started.
	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	f.assertNoActions()
}

func TestEventWatchManager_ignoresPreStartEvents(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	f.addManifest(mn)
	obj := f.makeObj(mn)
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{Name: obj.GetName()}: &obj,
	}

	f.ewm.OnChange(f.ctx, f.store)

	evt1 := f.makeEvent(obj)
	evt1.CreationTimestamp = metav1.Time{Time: f.clock.Now().Add(-time.Minute)}

	f.kClient.EmitEvent(f.ctx, evt1)

	evt2 := f.makeEvent(obj)

	f.kClient.EmitEvent(f.ctx, evt2)

	gvk := obj.GroupVersionKind()
	e := k8s.K8sEntity{
		Obj:  &obj,
		Kind: &gvk,
	}

	// first event predates tilt start time, so should be ignored
	expected := store.K8sEventAction{Event: evt2, ManifestName: mn, InvolvedObject: e}

	f.assertActions(expected)
}

func TestEventWatchManager_janitor(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("foo")

	f.addManifest(mn)

	obj1 := f.makeObj(mn)
	obj2 := f.makeObj(mn)
	f.kClient.GetResources = map[k8s.GetKey]*unstructured.Unstructured{
		k8s.GetKey{Name: obj1.GetName()}: &obj1,
		k8s.GetKey{Name: obj2.GetName()}: &obj2,
	}

	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, f.makeEvent(obj1))

	f.assertUIDMapKeys([]types.UID{obj1.GetUID()})

	f.clock.BlockUntil(1)
	f.clock.Advance(uidMapEntryTTL / 2)

	f.kClient.EmitEvent(f.ctx, f.makeEvent(obj2))
	f.assertUIDMapKeys([]types.UID{obj1.GetUID(), obj2.GetUID()})

	f.clock.BlockUntil(1)
	f.clock.Advance(uidMapEntryTTL/2 + 1)
	f.assertUIDMapKeys([]types.UID{obj2.GetUID()})
}

func TestGetGroup(t *testing.T) {
	for _, test := range []struct {
		name          string
		apiVersion    string
		expectedGroup string
	}{
		{"normal", "apps/v1", "apps"},
		// core types have an empty group
		{"core", "/v1", ""},
		// on some versions of k8s, deployment is buggy and doesn't have a version in its apiVersion
		{"no version", "extensions", "extensions"},
		{"alpha version", "apps/v1alpha1", "apps"},
		{"beta version", "apps/v1beta1", "apps"},
		// I've never seen this in the wild, but the docs say it's legal
		{"alpha version, no second number", "apps/v1alpha", "apps"},
	} {
		t.Run(test.name, func(t *testing.T) {
			obj := v1.ObjectReference{APIVersion: test.apiVersion}
			assert.Equal(t, test.expectedGroup, getGroup(obj))
		})
	}
}

func (f *ewmFixture) makeEvent(obj unstructured.Unstructured) *v1.Event {
	return &v1.Event{
		ObjectMeta:     metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: f.clock.Now()}},
		Reason:         "test event reason",
		Message:        "test event message",
		InvolvedObject: v1.ObjectReference{UID: obj.GetUID(), Name: obj.GetName()},
	}
}

func (f *ewmFixture) makeObj(mn model.ManifestName) unstructured.Unstructured {
	ret := unstructured.Unstructured{}

	f.objectCount++

	ret.SetName(fmt.Sprintf("obj%d", f.objectCount))
	ret.SetUID(types.UID(fmt.Sprintf("uid%d", f.objectCount)))
	ret.SetLabels(map[string]string{
		k8s.TiltRunIDLabel:    k8s.TiltRunID,
		k8s.ManifestNameLabel: mn.String(),
	})

	return ret
}

type ewmFixture struct {
	t          *testing.T
	kClient    *k8s.FakeK8sClient
	ewm        *EventWatchManager
	ctx        context.Context
	cancel     func()
	store      *store.Store
	getActions func() []store.Action
	clock      clockwork.FakeClock

	// old value of k8sEventsFeatureFlag env var, for teardown
	// TODO(maia): remove this when we remove the feature flag
	oldFeatureFlagVal string

	objectCount int
}

func newEWMFixture(t *testing.T) *ewmFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	clock := clockwork.NewFakeClock()

	ret := &ewmFixture{
		kClient: kClient,
		ewm:     NewEventWatchManager(kClient, clock),
		ctx:     ctx,
		cancel:  cancel,
		t:       t,
		clock:   clock,
	}

	ret.store, ret.getActions = store.NewStoreForTesting()
	state := ret.store.LockMutableStateForTesting()
	state.TiltStartTime = clock.Now()
	ret.store.UnlockMutableState()
	go ret.store.Loop(ctx)

	return ret
}

func (f *ewmFixture) TearDown() {
	f.kClient.TearDown()
	f.cancel()
}

func (f *ewmFixture) addManifest(manifestName model.ManifestName) {
	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true
	dt := model.K8sTarget{Name: model.TargetName(manifestName)}
	m := model.Manifest{Name: manifestName}.WithDeployTarget(dt)
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

func (f *ewmFixture) assertUIDMapKeys(expectedKeys []types.UID) {
	uidMapKeys := func() []types.UID {
		var ret []types.UID
		f.ewm.uidMapMu.Lock()
		for k := range f.ewm.uidMap {
			ret = append(ret, types.UID(k))
		}
		f.ewm.uidMapMu.Unlock()
		return ret
	}

	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		if len(uidMapKeys()) == len(expectedKeys) {
			break
		}
	}

	assert.ElementsMatch(f.t, expectedKeys, uidMapKeys())
}
