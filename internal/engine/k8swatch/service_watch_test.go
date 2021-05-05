package k8swatch

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/servicebuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestServiceWatch(t *testing.T) {
	f := newSWFixture(t)
	defer f.TearDown()

	nodePort := 9998
	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")

	ls := k8s.ManagedByTiltSelector()
	s := servicebuilder.New(f.t, manifest).
		WithPort(9998).
		WithNodePort(int32(nodePort)).
		WithIP(string(f.nip)).
		WithUID(uid).
		Build()
	f.addDeployedService(manifest, s)
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

	f.sw.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	ls := k8s.ManagedByTiltSelector()
	s := servicebuilder.New(f.t, manifest).
		WithUID(uid).
		Build()
	f.kClient.EmitService(ls, s)
	f.waitUntilServiceKnown(uid)

	f.addDeployedService(manifest, s)

	expectedSCA := ServiceChangeAction{
		Service:      s,
		ManifestName: manifest.Name,
	}
	f.assertObservedServiceChangeActions(expectedSCA)
}

func (f *swFixture) addManifest(manifestName model.ManifestName) model.Manifest {
	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()

	m := manifestbuilder.New(f, manifestName).
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m))
	return m
}

func (f *swFixture) addDeployedService(m model.Manifest, svc *v1.Service) {
	defer f.sw.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.K8sRuntimeState()
	runtimeState.DeployedEntities = k8s.ObjRefList{
		k8s.NewK8sEntity(svc).ToObjectReference(),
	}
	mState.RuntimeState = runtimeState
}

type swFixture struct {
	*tempdir.TempDirFixture
	t       *testing.T
	kClient *k8s.FakeK8sClient
	nip     k8s.NodeIP
	sw      *ServiceWatcher
	ctx     context.Context
	cancel  func()
	store   *store.TestingStore
}

func newSWFixture(t *testing.T) *swFixture {
	nip := k8s.NodeIP("fakeip")

	kClient := k8s.NewFakeK8sClient(t)
	kClient.FakeNodeIP = nip

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	of := k8s.ProvideOwnerFetcher(ctx, kClient)
	sw := NewServiceWatcher(kClient, of, k8s.DefaultNamespace)
	st := store.NewTestingStore()

	return &swFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		kClient:        kClient,
		sw:             sw,
		nip:            nip,
		ctx:            ctx,
		cancel:         cancel,
		t:              t,
		store:          st,
	}
}

func (f *swFixture) TearDown() {
	f.kClient.TearDown()
	f.cancel()
	f.store.AssertNoErrorActions(f.t)
}

func (f *swFixture) assertObservedServiceChangeActions(expectedSCAs ...ServiceChangeAction) {
	start := time.Now()
	for time.Since(start) < time.Second {
		actions := f.store.Actions()
		if len(actions) == len(expectedSCAs) {
			break
		}
	}

	var observedSCAs []ServiceChangeAction
	for _, a := range f.store.Actions() {
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
	for time.Since(start) < time.Second {
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
