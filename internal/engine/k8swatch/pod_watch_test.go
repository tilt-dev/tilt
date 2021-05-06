package k8swatch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/podbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

const stdTimeout = time.Second

type ancestorMap map[types.UID]types.UID

func TestPodWatch(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	p := pb.Build()

	// Simulate the deployed entities in the engine state
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.requireWatchForEntity(manifest.Name, entities.Deployment())
	f.kClient.Inject(entities...)

	f.kClient.UpsertPod(p)

	f.assertObservedPods(manifest.Name, ancestorMap{p.UID: entities.Deployment().UID()})
}

func TestPodWatchChangeEventBeforeUID(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	p := pb.Build()

	entities := pb.ObjectTreeEntities()
	f.kClient.Inject(entities...)
	// emit an event before this manifest knows of anything deployed
	f.kClient.UpsertPod(p)

	require.Never(t, func() bool {
		state := f.store.RLockState()
		defer f.store.RUnlockState()
		kd := state.KubernetesDiscoveries[KeyForManifest(manifest.Name)]
		return kd != nil && len(kd.Status.Pods) != 0
	}, time.Second/2, 20*time.Millisecond, "No pods should have been observed")

	// Simulate the deployed entities in the engine state after
	// the pod event.
	f.addDeployedEntity(manifest, entities.Deployment())

	f.assertObservedPods(manifest.Name, ancestorMap{p.UID: entities.Deployment().UID()})
}

// We had a bug where if newPod.resourceVersion < oldPod.resourceVersion (using string comparison!)
// then we'd ignore the new pod. This meant, e.g., once we got an update for resourceVersion "9", we'd
// ignore updates for resourceVersions "10" through "89" and "100" through "899"
func TestPodWatchResourceVersionStringLessThan(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest).WithResourceVersion("9")

	// Simulate the deployed entities in the engine state
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.kClient.Inject(entities...)

	p1 := pb.Build()
	f.kClient.UpsertPod(p1)

	f.assertObservedPods(manifest.Name, ancestorMap{p1.UID: entities.Deployment().UID()})

	p2 := pb.WithResourceVersion("10").WithTemplateSpecHash("abc123").Build()
	f.kClient.UpsertPod(p2)

	f.requireState(KeyForManifest(manifest.Name), func(kd *v1alpha1.KubernetesDiscovery) bool {
		if kd == nil {
			return false
		}
		return kd.Status.Pods[0].PodTemplateSpecHash == "abc123"
	}, "Pod for updated resource version was not observed")
}

func TestPodWatchExtraSelectors(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	ls1 := labels.Set{"foo": "bar"}
	ls2 := labels.Set{"baz": "quu"}
	manifest := f.addManifestWithSelectors("server", ls1, ls2)

	p := podbuilder.New(t, manifest).
		WithPodLabel("foo", "bar").
		WithUnknownOwner().
		Build()
	f.kClient.UpsertPod(p)

	// there should be NO ancestor since this was matched by labels
	f.assertObservedPods(manifest.Name, ancestorMap{p.UID: ""})
}

func TestPodWatchHandleSelectorChange(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	ls1 := labels.Set{"foo": "bar"}
	manifest := f.addManifestWithSelectors("server1", ls1)

	// p1 matches ls1 but not ls2
	p1 := podbuilder.New(t, manifest).
		WithPodLabel("foo", "bar").
		WithUnknownOwner().
		Build()
	f.kClient.UpsertPod(p1)

	// p2 is for a known deployment UID
	pb2 := podbuilder.New(t, manifest).WithPodName("pod2")
	p2 := pb2.Build()
	p2Entities := pb2.ObjectTreeEntities()
	f.kClient.Inject(p2Entities...)
	f.kClient.UpsertPod(p2)
	f.addDeployedEntity(manifest, p2Entities.Deployment())

	// p3 matches NEITHER ls1 nor ls2
	// (it has th same label key as ls1 but a non-matching value!)
	p3 := podbuilder.New(t, manifest).
		WithPodName("pod3").
		WithPodLabel("foo", "wrong-value").
		WithUnknownOwner().
		Build()
	f.kClient.UpsertPod(p3)

	// p4 matches ls2 but not ls1
	p4 := podbuilder.New(t, manifest).
		WithPodName("pod4").
		WithPodLabel("baz", "quu").
		WithUnknownOwner().
		Build()
	f.kClient.UpsertPod(p4)

	// p3 + p4 should not be observed
	f.assertObservedPods(manifest.Name, ancestorMap{
		p1.UID: "",                            // p1 matches ls1 -> no ancestor
		p2.UID: p2Entities.Deployment().UID(), // p2 matches deployed UID
	})

	// change the labels - now p1 will NOT match but p4 WILL match; others stay the same
	ls2 := labels.Set{"baz": "quu"}
	manifest = f.addManifestWithSelectors(manifest.Name, ls2)
	// (since we upserted in the store, need to re-attach p2)
	f.addDeployedEntity(manifest, p2Entities.Deployment())

	f.assertObservedPods(manifest.Name, ancestorMap{
		p2.UID: p2Entities.Deployment().UID(), // p2 matches deployed UID
		p4.UID: "",                            // p4 matches ls2 -> no ancestor
	})
}

func TestPodWatchReadd(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	p := pb.Build()
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.kClient.Inject(entities...)
	f.kClient.UpsertPod(p)

	f.assertObservedPods(manifest.Name, ancestorMap{p.UID: entities.Deployment().UID()})

	f.removeManifest("server")
	// the watch should be removed now
	require.Eventuallyf(t, func() bool {
		key := KeyForManifest(manifest.Name)
		return !f.pw.HasNamespaceWatch(key, k8s.DefaultNamespace) &&
			!f.pw.HasUIDWatch(key, entities.Deployment().UID())
	}, stdTimeout, 20*time.Millisecond, "Namespace watch was never removed")

	manifest = f.addManifestWithSelectors("server")

	f.addDeployedEntity(manifest, entities.Deployment())

	// Make sure the pods are re-broadcast.
	// Even though the pod didn't change when the manifest was
	// redeployed, we still need to broadcast the pod to make
	// sure it gets repopulated.
	f.assertObservedPods(manifest.Name, ancestorMap{p.UID: entities.Deployment().UID()})
}

func TestPodWatchDuplicates(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	m1 := f.addManifestWithSelectors("server1")
	m2 := f.addManifestWithSelectors("server2")
	l := labels.Set{"foo": "bar"}
	m3 := f.addManifestWithSelectors("server3", l)
	m4 := f.addManifestWithSelectors("server4", l)

	pb := podbuilder.New(t, m1).
		WithPodName("shared-pod").
		WithPodLabel("foo", "bar").
		WithNoTemplateSpecHash()
	p := pb.Build()
	entities := pb.ObjectTreeEntities()
	// m3 + m4 don't know about the deployment, just labels
	f.addDeployedEntity(m1, entities.Deployment())
	f.addDeployedEntity(m2, entities.Deployment())
	// only m1 is expected to watch the UID
	// TODO(milas): this is actually a quirk of ManifestSubscriber - move to dedicated test there
	f.requireWatchForEntity(m1.Name, entities.Deployment())

	f.kClient.Inject(entities...)
	f.kClient.UpsertPod(p)

	// m1 should see the Pod and map it back to its ancestor (deployment)
	f.assertObservedPods(m1.Name, ancestorMap{p.UID: entities.Deployment().UID()})
	// m2 should see nothing since ManifestSubscriber gave the UID claim to m1
	f.assertObservedPods(m2.Name, ancestorMap{})
	// m3 + m4 still see the pod! but have no concept of ancestor since it was a label match
	for _, mn := range []model.ManifestName{m3.Name, m4.Name} {
		f.assertObservedPods(mn, ancestorMap{p.UID: ""})
	}

	f.removeManifest(m1.Name)

	// the UID should get reassigned to m2
	// AND dispatch a Pod event since it's "new" (to m2)
	f.requireWatchForEntity(m2.Name, entities.Deployment())
	f.assertObservedPods(m2.Name, ancestorMap{p.UID: entities.Deployment().UID()})
	// m3 + m4 still see the pod! but have no concept of ancestor since it was a label match
	for _, mn := range []model.ManifestName{m3.Name, m4.Name} {
		f.assertObservedPods(mn, ancestorMap{p.UID: ""})
	}

	f.removeManifest(m2.Name)

	// NOTE: label matches do NOT get re-dispatched events for known pods currently,
	// 	so we need to re-emit a Pod event
	f.kClient.UpsertPod(p)

	// m3 + m4 still see the pod! but have no concept of ancestor since it was a label match
	for _, mn := range []model.ManifestName{m3.Name, m4.Name} {
		f.assertObservedPods(mn, ancestorMap{p.UID: ""})
	}
}

func TestPodWatchRestarts(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest).WithRestartCount(123)
	p := pb.Build()

	// Simulate the deployed entities in the engine state
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.requireWatchForEntity(manifest.Name, entities.Deployment())
	f.kClient.Inject(entities...)

	f.kClient.UpsertPod(p)

	f.requireState(KeyForManifest(manifest.Name), func(kd *v1alpha1.KubernetesDiscovery) bool {
		if kd == nil || len(kd.Status.Pods) != 1 {
			return false
		}
		return kd.Status.Pods[0].BaselineRestartCount == 123
	}, "Incorrect restart count")
}

func (f *pwFixture) addManifestWithSelectors(mn model.ManifestName, ls ...labels.Set) model.Manifest {
	state := f.store.LockMutableStateForTesting()
	m := manifestbuilder.New(f, mn).
		WithK8sYAML(testyaml.SanchoYAML).
		WithK8sPodSelectors(ls).
		Build()
	mt := store.NewManifestTarget(m)
	state.UpsertManifestTarget(mt)
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())

	// ensure the change has propagated - manifest subscriber dispatches actions which once handled
	// should trigger the OnChange for the PodWatcher
	require.Eventuallyf(f.t, func() bool {
		// SanchoYAML doesn't define a namespace so it'll be the default
		return f.pw.HasNamespaceWatch(KeyForManifest(mn), k8s.DefaultNamespace)
	}, stdTimeout, 20*time.Millisecond, "Namespace for %q not being watched", mn)
	f.waitForExtraSelectors(mn, ls...)

	return mt.Manifest
}

func (f *pwFixture) waitForExtraSelectors(mn model.ManifestName, expected ...labels.Set) {
	var desc strings.Builder
	require.Eventuallyf(f.t, func() bool {
		desc.Reset()
		actualSelectors := f.pw.ExtraSelectors(KeyForManifest(mn))
		if len(expected) != len(actualSelectors) {
			desc.WriteString(fmt.Sprintf("expected selector count: %d | actual selector count: %d",
				len(expected), len(actualSelectors)))
			return false
		}

		selectorsEqual := true
		for selectorIndex := range expected {
			expectedReqs, _ := expected[selectorIndex].AsSelector().Requirements()
			actualReqs, _ := actualSelectors[selectorIndex].Requirements()
			for i := range expectedReqs {
				diff := cmp.Diff(expectedReqs[i].String(), actualReqs[i].String())
				if diff != "" {
					desc.WriteString("\n")
					desc.WriteString(diff)
					selectorsEqual = false
				}
			}
		}

		return selectorsEqual
	}, stdTimeout, 20*time.Millisecond, "Selectors never setup for %q: %s", mn, &desc)

}

func (f *pwFixture) removeManifest(mn model.ManifestName) {
	// before removing, take note of which namespaces are currently on the spec
	// for the relevant KubernetesDiscovery object so that we can assert they
	// get removed, both to ensure things are working as expected and to avoid
	// race conditions
	namespaces := make(map[k8s.Namespace]bool)

	key := KeyForManifest(mn)
	state := f.store.LockMutableStateForTesting()
	if kd := state.KubernetesDiscoveries[key]; kd != nil {
		for _, w := range kd.Spec.Watches {
			namespaces[k8s.Namespace(w.Namespace)] = true
		}
	}
	state.RemoveManifestTarget(mn)
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.requireState(KeyForManifest(mn), func(kd *v1alpha1.KubernetesDiscovery) bool {
		return kd == nil
	}, "KubernetesDiscovery was not removed from state for manifest[%s]", mn)
	// since spec has been removed from state, safe to synchronously assert that watches were removed
	for ns := range namespaces {
		require.Falsef(f.t, f.pw.HasNamespaceWatch(key, ns),
			"Watch for namespace[%s] by manifest[%s] still exists", ns, mn)
	}
}

type pwFixture struct {
	*tempdir.TempDirFixture
	t       *testing.T
	kClient *k8s.FakeK8sClient
	ms      *ManifestSubscriber
	pw      *PodWatcher
	ctx     context.Context
	cancel  func()
	store   *store.Store
	mu      sync.Mutex
}

func (pw *pwFixture) reducer(ctx context.Context, state *store.EngineState, action store.Action) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	switch a := action.(type) {
	case KubernetesDiscoveryCreateAction:
		HandleKubernetesDiscoveryCreateAction(ctx, state, a)
	case KubernetesDiscoveryUpdateAction:
		HandleKubernetesDiscoveryUpdateAction(ctx, state, a)
	case KubernetesDiscoveryDeleteAction:
		HandleKubernetesDiscoveryDeleteAction(ctx, state, a)
	case KubernetesDiscoveryUpdateStatusAction:
		HandleKubernetesDiscoveryUpdateStatusAction(ctx, state, a)
	case store.ErrorAction:
		pw.t.Fatalf("Store received ErrorAction: %v", a.Error)
	case store.PanicAction:
		pw.t.Fatalf("Store received PanicAction: %v", a.Err)
	default:
		pw.t.Fatalf("Unexpected action type: %T", action)
	}
}

func newPWFixture(t *testing.T) *pwFixture {
	kClient := k8s.NewFakeK8sClient(t)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	cli := fake.NewTiltClient()
	of := k8s.ProvideOwnerFetcher(ctx, kClient)
	rd := NewContainerRestartDetector()
	pw := NewPodWatcher(kClient, of, rd, cli)

	ms := NewManifestSubscriber(k8s.DefaultNamespace, cli)

	ret := &pwFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		kClient:        kClient,
		ms:             ms,
		pw:             pw,
		ctx:            ctx,
		cancel:         cancel,
		t:              t,
	}

	st := store.NewStore(ret.reducer, false)
	require.NoError(t, st.AddSubscriber(ctx, ms))
	require.NoError(t, st.AddSubscriber(ctx, pw))

	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

	ret.store = st

	return ret
}

func (f *pwFixture) TearDown() {
	f.TempDirFixture.TearDown()
	f.kClient.TearDown()
	f.cancel()
}

func (f *pwFixture) addDeployedEntity(m model.Manifest, entity k8s.K8sEntity) {
	f.t.Helper()

	state := f.store.LockMutableStateForTesting()
	mState, ok := state.ManifestState(m.Name)
	if !ok {
		f.t.Fatalf("Unknown manifest: %s", m.Name)
	}
	runtimeState := mState.K8sRuntimeState()
	runtimeState.DeployedEntities = k8s.ObjRefList{entity.ToObjectReference()}
	mState.RuntimeState = runtimeState
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
}

func (f *pwFixture) requireWatchForEntity(mn model.ManifestName, entity k8s.K8sEntity) {
	key := KeyForManifest(mn)
	require.Eventuallyf(f.t, func() bool {
		return f.pw.HasUIDWatch(key, entity.UID())
	}, stdTimeout, 20*time.Millisecond, "Watch for manifest[%s] on entity[%s] not setup", mn, entity.Name())
}

func (f *pwFixture) assertObservedPods(mn model.ManifestName, expected ancestorMap) {
	f.t.Helper()
	var desc strings.Builder
	f.requireState(KeyForManifest(mn), func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("KubernetesDiscovery object is nil in state")
			return false
		}
		actual := make(ancestorMap)
		for _, p := range kd.Status.Pods {
			podUID := types.UID(p.UID)
			actual[podUID] = types.UID(p.AncestorUID)
		}

		diff := cmp.Diff(expected, actual)
		if diff != "" {
			desc.WriteString("\n")
			desc.WriteString(diff)
			return false
		}
		return true
	}, "Expected Pods were not observed for manifest[%s]: %s", mn, &desc)
}

func (f *pwFixture) requireState(key types.NamespacedName, cond func(kd *v1alpha1.KubernetesDiscovery) bool, msg string, args ...interface{}) {
	f.t.Helper()
	require.Eventuallyf(f.t, func() bool {
		state := f.store.RLockState()
		defer f.store.RUnlockState()
		return cond(state.KubernetesDiscoveries[key])
	}, stdTimeout, 20*time.Millisecond, msg, args...)
}
