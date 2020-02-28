package k8swatch

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/servicebuilder"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestServiceWatch(t *testing.T) {
	f := newSWFixture(t)
	defer f.TearDown()

	nodePort := 9998
	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")
	f.addDeployedUID(manifest, uid)

	ls := k8s.ManagedByTiltSelector()
	s := servicebuilder.New(f.t, manifest).
		WithPort(9998).
		WithNodePort(int32(nodePort)).
		WithIP(string(f.nip)).
		WithUID(uid).
		Build()
	f.kClient.EmitService(ls, s)

	expectedSCA := ServiceChangeAction{
		Service:      s,
		ManifestName: manifest.Name,
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", f.nip, nodePort),
			Path:   "/",
		},
	}

	f.assertObservedServiceChangeActions(expectedSCA)
}

// In many environments, we will get a Service change event
// faster than the `kubectl apply` finishes. So we need to hold onto
// the Service and dispatch an event when the UID returned by `kubectl apply`
// shows up.
func TestServiceWatchUIDDelayed(t *testing.T) {
	f := newSWFixture(t)
	defer f.TearDown()

	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")

	f.sw.OnChange(f.ctx, f.store)

	ls := k8s.ManagedByTiltSelector()
	s := servicebuilder.New(f.t, manifest).
		WithUID(uid).
		Build()
	f.kClient.EmitService(ls, s)
	f.waitUntilServiceKnown(uid)

	f.addDeployedUID(manifest, uid)

	expectedSCA := ServiceChangeAction{
		Service:      s,
		ManifestName: manifest.Name,
	}
	f.assertObservedServiceChangeActions(expectedSCA)
}

func (f *swFixture) addManifest(manifestName string) model.Manifest {
	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	state.WatchFiles = true
	dt := model.K8sTarget{Name: model.TargetName(manifestName)}
	m := model.Manifest{Name: model.ManifestName(manifestName)}.WithDeployTarget(dt)
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	return m
}

func (f *swFixture) addDeployedUID(m model.Manifest, uid types.UID) {
	defer f.sw.OnChange(f.ctx, f.store)

	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.GetOrCreateK8sRuntimeState()
	runtimeState.DeployedUIDSet[uid] = true
}

type swFixture struct {
	t          *testing.T
	kClient    *k8s.FakeK8sClient
	nip        k8s.NodeIP
	sw         *ServiceWatcher
	ctx        context.Context
	cancel     func()
	store      *store.Store
	getActions func() []store.Action
}

func newSWFixture(t *testing.T) *swFixture {
	nip := k8s.NodeIP("fakeip")

	kClient := k8s.NewFakeK8sClient()
	kClient.FakeNodeIP = nip

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	of := k8s.ProvideOwnerFetcher(kClient)
	sw := NewServiceWatcher(kClient, of)

	ret := &swFixture{
		kClient: kClient,
		sw:      sw,
		nip:     nip,
		ctx:     ctx,
		cancel:  cancel,
		t:       t,
	}

	ret.store, ret.getActions = store.NewStoreForTesting()
	go func() {
		err := ret.store.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

	return ret
}

func (f *swFixture) TearDown() {
	f.kClient.TearDown()
	f.cancel()
}

func (f *swFixture) assertObservedServiceChangeActions(expectedSCAs ...ServiceChangeAction) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		actions := f.getActions()
		if len(actions) == len(expectedSCAs) {
			break
		}
	}

	var observedSCAs []ServiceChangeAction
	for _, a := range f.getActions() {
		sca, ok := a.(ServiceChangeAction)
		if !ok {
			f.t.Fatalf("got non-%T: %v", ServiceChangeAction{}, a)
		}
		observedSCAs = append(observedSCAs, sca)
	}
	if !assert.Equal(f.t, expectedSCAs, observedSCAs) {
		f.t.FailNow()
	}
}

func (f *swFixture) waitUntilServiceKnown(uid types.UID) {
	start := time.Now()
	for time.Since(start) < 200*time.Millisecond {
		f.sw.mu.Lock()
		_, known := f.sw.knownServices[uid]
		f.sw.mu.Unlock()
		if known {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	f.t.Fatalf("timeout waiting for service with UID: %s", uid)
}
