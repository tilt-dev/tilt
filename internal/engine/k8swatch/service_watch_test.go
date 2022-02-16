package k8swatch

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/servicebuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestServiceWatch(t *testing.T) {
	f := newSWFixture(t)

	nodePort := 9998
	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")

	s := servicebuilder.New(f.t, manifest).
		WithPort(9998).
		WithNodePort(int32(nodePort)).
		WithIP(string(f.nip)).
		WithUID(uid).
		Build()
	f.addDeployedService(manifest, s)
	f.kClient.UpsertService(s)

	require.NoError(f.t, f.sw.OnChange(f.ctx, f.store, store.LegacyChangeSummary()))

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

	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")

	// the watcher won't start until it has a deployed object ref to find a namespace to watch in
	// so we need to create at least one first
	dummySvc := servicebuilder.New(t, manifest).WithUID("placeholder").Build()
	f.kClient.UpsertService(dummySvc)
	f.addDeployedService(manifest, dummySvc)

	_ = f.sw.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	// this service should be seen even by the watcher even though it's not yet referenced by the manifest
	s := servicebuilder.New(f.t, manifest).
		WithUID(uid).
		Build()
	f.kClient.UpsertService(s)
	f.waitUntilServiceKnown(uid)

	// once it's referenced by the manifest, an event should get emitted
	f.addDeployedService(manifest, s)
	expected := []ServiceChangeAction{
		{
			Service:      dummySvc,
			ManifestName: manifest.Name,
		},
		{
			Service:      s,
			ManifestName: manifest.Name,
		},
	}
	f.assertObservedServiceChangeActions(expected...)
}

func TestServiceWatchClusterChange(t *testing.T) {
	f := newSWFixture(t)

	port := int32(1234)
	uid := types.UID("fake-uid")
	manifest := f.addManifest("server")

	s := servicebuilder.New(f.t, manifest).
		WithPort(port).
		WithNodePort(9998).
		WithIP(string(f.nip)).
		WithUID(uid).
		Build()
	f.addDeployedService(manifest, s)
	f.kClient.UpsertService(s)

	expectedSCA := ServiceChangeAction{
		Service:      s,
		ManifestName: manifest.Name,
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", f.nip, port),
			Path:   "/",
		},
	}

	f.assertObservedServiceChangeActions(expectedSCA)
	f.store.ClearActions()

	newClusterClient := k8s.NewFakeK8sClient(t)
	newSvc := s.DeepCopy()
	port = 4567
	newSvc.Spec.Ports[0].NodePort = 9997
	newSvc.Spec.Ports[0].Port = port
	newClusterClient.UpsertService(newSvc)
	clusterNN := types.NamespacedName{Name: "default"}
	// add the new client to
	f.clients.SetK8sClient(clusterNN, newClusterClient)
	_, createdAt, err := f.clients.GetK8sClient(clusterNN)
	require.NoError(t, err, "Could not get cluster client hash")
	state := f.store.LockMutableStateForTesting()
	state.Clusters["default"].Status.ConnectedAt = createdAt.DeepCopy()
	f.store.UnlockMutableState()

	err = f.sw.OnChange(f.ctx, f.store, store.ChangeSummary{
		Clusters: store.NewChangeSet(clusterNN),
	})
	require.NoError(t, err, "OnChange failed")
	f.assertObservedServiceChangeActions(ServiceChangeAction{
		Service:      newSvc,
		ManifestName: manifest.Name,
		URL: &url.URL{
			Scheme: "http",
			Host:   fmt.Sprintf("%s:%d", f.nip, port),
			Path:   "/",
		},
	})
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
	defer func() {
		require.NoError(f.t, f.sw.OnChange(f.ctx, f.store, store.LegacyChangeSummary()))
	}()

	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.K8sRuntimeState()
	runtimeState.ApplyFilter = &k8sconv.KubernetesApplyFilter{
		DeployedRefs: k8s.ObjRefList{k8s.NewK8sEntity(svc).ToObjectReference()},
	}
	mState.RuntimeState = runtimeState
}

type swFixture struct {
	*tempdir.TempDirFixture
	t       *testing.T
	clients *cluster.FakeClientProvider
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

	clients := cluster.NewFakeClientProvider(kClient)
	sw := NewServiceWatcher(clients, k8s.DefaultNamespace)
	st := store.NewTestingStore()

	state := st.LockMutableStateForTesting()
	_, createdAt, err := clients.GetK8sClient(types.NamespacedName{Name: "default"})
	require.NoError(t, err, "Failed to get default cluster client hash")
	state.Clusters["default"] = &v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
		Status: v1alpha1.ClusterStatus{
			Arch:        "fake-arch",
			ConnectedAt: createdAt.DeepCopy(),
		},
	}
	st.UnlockMutableState()

	ret := &swFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		clients:        clients,
		kClient:        kClient,
		sw:             sw,
		nip:            nip,
		ctx:            ctx,
		cancel:         cancel,
		t:              t,
		store:          st,
	}

	t.Cleanup(ret.TearDown)

	return ret
}

func (f *swFixture) TearDown() {
	f.cancel()
	f.store.AssertNoErrorActions(f.t)
}

func (f *swFixture) assertObservedServiceChangeActions(expectedSCAs ...ServiceChangeAction) {
	f.t.Helper()
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
	clusterNN := types.NamespacedName{Name: v1alpha1.ClusterNameDefault}
	start := time.Now()
	for time.Since(start) < time.Second {
		f.sw.mu.Lock()
		_, known := f.sw.knownServices[clusterUID{cluster: clusterNN, uid: uid}]
		f.sw.mu.Unlock()
		if known {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	f.t.Fatalf("timeout waiting for service with UID: %s", uid)
}
