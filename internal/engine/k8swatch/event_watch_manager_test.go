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

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
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

	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.kClient.EmitEvent(f.ctx, evt)
	expected := store.K8sEventAction{Event: evt, ManifestName: mn}
	f.assertActions(expected)
}

func TestEventWatchManager_dispatchesNamespaceEvent(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	// Seed the k8s client with a pod and its owner tree
	manifest := f.addManifest(mn)
	pb := podbuilder.New(t, manifest)
	f.addDeployedUID(manifest, pb.DeploymentUID())
	f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

	evt1 := f.makeEvent(k8s.NewK8sEntity(pb.Build()))
	evt1.ObjectMeta.Namespace = "kube-system"

	evt2 := f.makeEvent(k8s.NewK8sEntity(pb.Build()))

	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.kClient.EmitEvent(f.ctx, evt1)
	f.kClient.EmitEvent(f.ctx, evt2)

	expected := store.K8sEventAction{Event: evt2, ManifestName: mn}
	f.assertActions(expected)
}

func TestEventWatchManager_duplicateDeployIDs(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	fe1 := model.ManifestName("fe1")
	m1 := f.addManifest(fe1)
	fe2 := model.ManifestName("fe2")
	m2 := f.addManifest(fe2)

	// Seed the k8s client with a pod and its owner tree
	pb := podbuilder.New(t, m1)
	f.addDeployedUID(m1, pb.DeploymentUID())
	f.addDeployedUID(m2, pb.DeploymentUID())
	f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

	evt := f.makeEvent(k8s.NewK8sEntity(pb.Build()))

	f.kClient.EmitEvent(f.ctx, evt)
	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	expected := store.K8sEventAction{Event: evt, ManifestName: fe1}
	f.assertActions(expected)
}

type eventTestCase struct {
	Reason   string
	Type     string
	Expected bool
}

func TestEventWatchManagerDifferentEvents(t *testing.T) {
	cases := []eventTestCase{
		eventTestCase{Reason: "Bumble", Type: v1.EventTypeNormal, Expected: false},
		eventTestCase{Reason: "Bumble", Type: v1.EventTypeWarning, Expected: true},
		eventTestCase{Reason: ImagePulledReason, Type: v1.EventTypeNormal, Expected: true},
		eventTestCase{Reason: ImagePullingReason, Type: v1.EventTypeNormal, Expected: true},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("Case%d", i), func(t *testing.T) {
			f := newEWMFixture(t)
			defer f.TearDown()

			mn := model.ManifestName("someK8sManifest")

			// Seed the k8s client with a pod and its owner tree
			manifest := f.addManifest(mn)
			pb := podbuilder.New(t, manifest)
			f.addDeployedUID(manifest, pb.DeploymentUID())
			f.kClient.InjectEntityByName(pb.ObjectTreeEntities()...)

			evt := f.makeEvent(k8s.NewK8sEntity(pb.Build()))
			evt.Reason = c.Reason
			evt.Type = c.Type

			f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
			f.kClient.EmitEvent(f.ctx, evt)
			if c.Expected {
				expected := store.K8sEventAction{Event: evt, ManifestName: mn}
				f.assertActions(expected)
			} else {
				f.assertNoActions()
			}
		})
	}
}

func TestEventWatchManager_listensOnce(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	f.addManifest("fe")
	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	f.kClient.EventsWatchErr = fmt.Errorf("Multiple watches forbidden")
	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.assertActions()
}

func TestEventWatchManager_watchError(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	err := fmt.Errorf("oh noes")
	f.kClient.EventsWatchErr = err
	f.addManifest("someK8sManifest")

	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	expectedErr := errors.Wrap(err, "Error watching events. Are you connected to kubernetes?\nTry running `kubectl get events -n \"default\"`")
	expected := store.ErrorAction{Error: expectedErr}
	f.assertActions(expected)
	f.store.ClearActions()
}

func TestEventWatchManager_eventBeforeUID(t *testing.T) {
	f := newEWMFixture(t)
	defer f.TearDown()

	mn := model.ManifestName("someK8sManifest")

	// Seed the k8s client with a pod and its owner tree
	manifest := f.addManifest(mn)
	f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

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
		ObjectMeta: metav1.ObjectMeta{
			CreationTimestamp: metav1.Time{Time: f.clock.Now()},
			Namespace:         k8s.DefaultNamespace.String(),
		},
		Reason:         "test event reason",
		Message:        "test event message",
		InvolvedObject: v1.ObjectReference{UID: obj.UID(), Name: obj.Name()},
		Type:           v1.EventTypeWarning,
	}
}

type ewmFixture struct {
	*tempdir.TempDirFixture
	t       *testing.T
	kClient *k8s.FakeK8sClient
	ewm     *EventWatchManager
	ctx     context.Context
	cancel  func()
	store   *store.TestingStore
	clock   clockwork.FakeClock
}

func newEWMFixture(t *testing.T) *ewmFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	of := k8s.ProvideOwnerFetcher(ctx, kClient)

	clock := clockwork.NewFakeClock()
	st := store.NewTestingStore()

	ret := &ewmFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		kClient:        kClient,
		ewm:            NewEventWatchManager(kClient, of, k8s.DefaultNamespace),
		ctx:            ctx,
		cancel:         cancel,
		t:              t,
		clock:          clock,
		store:          st,
	}

	state := ret.store.LockMutableStateForTesting()
	state.TiltStartTime = clock.Now()
	ret.store.UnlockMutableState()

	return ret
}

func (f *ewmFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
	f.kClient.TearDown()
	f.store.AssertNoErrorActions(f.t)
}

func (f *ewmFixture) addManifest(manifestName model.ManifestName) model.Manifest {
	state := f.store.LockMutableStateForTesting()

	m := manifestbuilder.New(f, manifestName).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	f.store.UnlockMutableState()
	return m
}

func (f *ewmFixture) addDeployedUID(m model.Manifest, uid types.UID) {
	defer f.ewm.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.K8sRuntimeState()
	runtimeState.DeployedUIDSet[uid] = true
}

func (f *ewmFixture) assertNoActions() {
	f.assertActions()
}

func (f *ewmFixture) assertActions(expected ...store.Action) {
	start := time.Now()
	for time.Since(start) < time.Second {
		actions := f.store.Actions()
		if len(actions) >= len(expected) {
			break
		}
	}

	// Make extra sure we didn't get any extra actions
	time.Sleep(10 * time.Millisecond)

	// NOTE(maia): this test will break if this the code ever returns other
	// correct-but-incidental-to-this-test actions, but for now it's fine.
	actual := f.store.Actions()
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
