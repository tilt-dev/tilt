package k8swatch

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
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/podbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestEventWatchManager_dispatchesEvent(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	// Seed the k8s client with a pod and its owner tree
	manifest := f.addManifest(mn)
	pb := podbuilder.New(t, manifest)
	f.addDeployedUID(manifest, pb.DeploymentUID())
	f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

	evt := f.makeEvent(k8s.NewK8sEntity(pb.Build()))

	f.ewm.OnChange(f.ctx, f.store)
	f.kClient.EmitEvent(f.ctx, evt)
	expected := store.K8sEventAction{Event: evt, ManifestName: mn}
	f.assertActions(expected)
}

func TestEventWatchManager_listensOnce(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	f.addManifest("fe")
	f.ewm.OnChange(f.ctx, f.store)

	f.kClient.EventsWatchErr = fmt.Errorf("Multiple watches forbidden")
	f.ewm.OnChange(f.ctx, f.store)
	f.assertActions()
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
	f.storeLoopErr = nil
}

func TestEventWatchManager_eventBeforeUID(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	// Seed the k8s client with a pod and its owner tree
	manifest := f.addManifest(mn)
	pb := podbuilder.New(t, manifest)
	f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

	evt := f.makeEvent(k8s.NewK8sEntity(pb.Build()))

	// The UIDs haven't shown up in the engine state yet, so
	// we shouldn't emit the events.
	f.kClient.EmitEvent(f.ctx, evt)
	f.assertNoActions()

	// When the UIDs of the deployed objects show up, then
	// we need to go back and emit the events we saw earlier.
	f.addDeployedUID(manifest, pb.DeploymentUID())
	expected := store.K8sEventAction{Event: evt, ManifestName: mn}
	f.assertActions(expected)
}

func TestEventWatchManager_ignoresPreStartEvents(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	// Seed the k8s client with a pod and its owner tree
	manifest := f.addManifest(mn)
	pb := podbuilder.New(t, manifest)
	f.addDeployedUID(manifest, pb.DeploymentUID())
	f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

	entity := k8s.NewK8sEntity(pb.Build())
	evt1 := f.makeEvent(entity)
	evt1.CreationTimestamp = metav1.Time{Time: f.clock.Now().Add(-time.Minute)}

	f.kClient.EmitEvent(f.ctx, evt1)

	evt2 := f.makeEvent(entity)

	f.kClient.EmitEvent(f.ctx, evt2)

	// first event predates tilt start time, so should be ignored
	expected := store.K8sEventAction{Event: evt2, ManifestName: mn}

	f.assertActions(expected)
}

func (f *ewmFixture) makeEvent(obj k8s.K8sEntity) *v1.Event {
	return &v1.Event{
		ObjectMeta:     metav1.ObjectMeta{CreationTimestamp: metav1.Time{Time: f.clock.Now()}},
		Reason:         "test event reason",
		Message:        "test event message",
		InvolvedObject: v1.ObjectReference{UID: obj.UID(), Name: obj.Name()},
	}
}

type ewmFixture struct {
	*tempdir.TempDirFixture
	t            *testing.T
	kClient      *k8s.FakeK8sClient
	ewm          *EventWatchManager
	ctx          context.Context
	cancel       func()
	store        *store.Store
	storeLoopErr error
	getActions   func() []store.Action
	clock        clockwork.FakeClock
}

func newEWMFixture(t *testing.T) *ewmFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	of := k8s.ProvideOwnerFetcher(kClient)

	clock := clockwork.NewFakeClock()

	ret := &ewmFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		kClient:        kClient,
		ewm:            NewEventWatchManager(kClient, of),
		ctx:            ctx,
		cancel:         cancel,
		t:              t,
		clock:          clock,
	}

	ret.store, ret.getActions = store.NewStoreForTesting()
	state := ret.store.LockMutableStateForTesting()
	state.TiltStartTime = clock.Now()
	ret.store.UnlockMutableState()
	go func() {
		ret.storeLoopErr = ret.store.Loop(ctx)
	}()

	return ret
}

func (f *ewmFixture) TearDown() {
	testutils.FailOnNonCanceledErr(f.t, f.storeLoopErr, "store.Loop returned an error")
	f.TempDirFixture.TearDown()
	f.kClient.TearDown()
	f.cancel()
}

func (f *ewmFixture) addManifest(manifestName model.ManifestName) model.Manifest {
	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true

	m := manifestbuilder.New(f, manifestName).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()
	return m
}

func (f *ewmFixture) addDeployedUID(m model.Manifest, uid types.UID) {
	defer f.ewm.OnChange(f.ctx, f.store)

	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.GetOrCreateK8sRuntimeState()
	runtimeState.DeployedUIDSet[uid] = true
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
