package k8swatch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
	f.kClient.InjectEntityByName(entities...)

	f.kClient.EmitPod(labels.Everything(), p)

	f.assertObservedPods(p)
}

func TestPodWatchChangeEventBeforeUID(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	p := pb.Build()

	entities := pb.ObjectTreeEntities()
	f.kClient.InjectEntityByName(entities...)
	// emit an event before this manifest knows of anything deployed
	f.kClient.EmitPod(labels.Everything(), p)

	require.Never(t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.pods) != 0
	}, time.Second/2, 20*time.Millisecond, "No pods should have been observed")

	// Simulate the deployed entities in the engine state after
	// the pod event.
	f.addDeployedEntity(manifest, entities.Deployment())

	f.assertObservedPods(p)
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
	f.kClient.InjectEntityByName(entities...)

	p1 := pb.Build()
	f.kClient.EmitPod(labels.Everything(), p1)

	f.assertObservedPods(p1)

	p2 := pb.WithResourceVersion("10").Build()
	f.kClient.EmitPod(labels.Everything(), p2)

	f.assertObservedPods(p1, p2)
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
	f.kClient.EmitPod(labels.Everything(), p)

	f.assertObservedPods(p)
	f.assertObservedManifests(manifest.Name)
}

func TestPodWatchHandleSelectorChange(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	ls1 := labels.Set{"foo": "bar"}
	manifest := f.addManifestWithSelectors("server1", ls1)

	p := podbuilder.New(t, manifest).
		WithPodLabel("foo", "bar").
		WithUnknownOwner().
		Build()
	f.kClient.EmitPod(labels.Everything(), p)

	f.assertObservedPods(p)
	f.clearPods()

	ls2 := labels.Set{"baz": "quu"}
	manifest2 := f.addManifestWithSelectors("server2", ls2)

	// remove the first manifest and wait it to propagate
	f.removeManifest(manifest.Name)

	pb2 := podbuilder.New(t, manifest2).WithPodID("pod2")
	p2 := pb2.Build()
	p2Entities := pb2.ObjectTreeEntities()
	f.addDeployedEntity(manifest2, p2Entities.Deployment())
	f.kClient.InjectEntityByName(p2Entities...)
	f.kClient.EmitPod(labels.Everything(), p2)
	f.assertObservedPods(p2)
	f.clearPods()

	p3 := podbuilder.New(t, manifest2).
		WithPodID("pod3").
		WithPodLabel("foo", "bar").
		WithUnknownOwner().
		Build()
	f.kClient.EmitPod(labels.Everything(), p3)

	p4 := podbuilder.New(t, manifest2).
		WithPodID("pod4").
		WithPodLabel("baz", "quu").
		WithUnknownOwner().
		Build()
	f.kClient.EmitPod(labels.Everything(), p4)

	p5 := podbuilder.New(t, manifest2).
		WithPodID("pod5").
		Build()
	f.kClient.EmitPod(labels.Everything(), p5)

	f.assertObservedPods(p4, p5)
	assert.Equal(t, []model.ManifestName{manifest2.Name, manifest2.Name}, f.manifestNames)
}

func TestPodsDispatchedInOrder(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()
	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.requireWatchForEntity(manifest.Name, entities.Deployment())
	f.kClient.InjectEntityByName(entities...)

	count := 20
	pods := []*v1.Pod{}
	for i := 0; i < count; i++ {
		v := strconv.Itoa(i)
		pod := pb.
			WithResourceVersion(v).
			WithTemplateSpecHash(k8s.PodTemplateSpecHash(v)).
			Build()
		pods = append(pods, pod)
	}

	for _, pod := range pods {
		f.kClient.EmitPod(labels.Everything(), pod)
	}

	f.waitForPodActionCount(count)

	// Make sure the pods showed up in order.
	for i := 1; i < count; i++ {
		pod := f.pods[i]
		lastPod := f.pods[i-1]
		podV, _ := strconv.Atoi(pod.PodTemplateSpecHash)
		lastPodV, _ := strconv.Atoi(lastPod.PodTemplateSpecHash)
		if lastPodV > podV {
			t.Fatalf("Pods appeared out of order\nPod %d: %v\nPod %d: %v\n", i-1, lastPod, i, pod)
		}
	}
}

func TestPodWatchReadd(t *testing.T) {
	f := newPWFixture(t)
	defer f.TearDown()

	manifest := f.addManifestWithSelectors("server")

	pb := podbuilder.New(t, manifest)
	p := pb.Build()
	entities := pb.ObjectTreeEntities()
	f.addDeployedEntity(manifest, entities.Deployment())
	f.kClient.InjectEntityByName(entities...)
	f.kClient.EmitPod(labels.Everything(), p)

	f.assertObservedPods(p)

	f.removeManifest("server")
	// the watch should be removed now
	require.Eventuallyf(t, func() bool {
		return !f.pw.HasNamespaceWatch(keyForManifest(manifest.Name), k8s.DefaultNamespace)
	}, stdTimeout, 20*time.Millisecond, "Namespace watch was never removed")

	f.clearPods()
	manifest = f.addManifestWithSelectors("server")

	f.addDeployedEntity(manifest, pb.ObjectTreeEntities().Deployment())

	// Make sure the pods are re-broadcast.
	// Even though the pod didn't change when the manifest was
	// redeployed, we still need to broadcast the pod to make
	// sure it gets repopulated.
	f.assertObservedPods(p)
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
		WithPodID("shared-pod").
		WithPodLabel("foo", "bar")
	p := pb.Build()
	entities := pb.ObjectTreeEntities()
	// m3 + m4 don't know about the deployment, just labels
	f.addDeployedEntity(m1, entities.Deployment())
	f.addDeployedEntity(m2, entities.Deployment())
	// only m1 is expected to watch the UID
	f.requireWatchForEntity(m1.Name, entities.Deployment())

	f.kClient.InjectEntityByName(entities...)
	f.kClient.EmitPod(labels.Everything(), p)

	f.assertObservedManifests(m1.Name)
	f.assertObservedPods(p)

	f.clearPods()
	f.removeManifest(m1.Name)

	// the UID should get reassigned to m2
	// AND dispatch a Pod event since it's "new" (to m2)
	f.requireWatchForEntity(m2.Name, entities.Deployment())
	f.assertObservedManifests(m2.Name)
	f.assertObservedPods(p)

	f.clearPods()
	f.removeManifest(m2.Name)

	// NOTE: label matches do NOT get re-dispatched events for known pods currently,
	// 	so we re-emit a Pod event
	f.kClient.EmitPod(labels.Everything(), p)

	// m3 should now be allowed to match, but m4 still shouldn't because
	// we restrict label matches to one watcher
	f.assertObservedManifests(m3.Name)
	f.assertObservedPods(p)

	f.clearPods()
	f.removeManifest(m3.Name)

	// NOTE: label matches do NOT get re-dispatched events for known pods currently,
	// 	so we re-emit a Pod event
	f.kClient.EmitPod(labels.Everything(), p)

	// finally, m4 can see it
	f.assertObservedManifests(m4.Name)
	f.assertObservedPods(p)
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
		return f.pw.HasNamespaceWatch(keyForManifest(mn), k8s.DefaultNamespace)
	}, stdTimeout, 20*time.Millisecond, "Namespace for %q not being watched", mn)
	f.waitForExtraSelectors(mn, ls...)

	return mt.Manifest
}

func (f *pwFixture) waitForExtraSelectors(mn model.ManifestName, expected ...labels.Set) {
	var desc strings.Builder
	require.Eventuallyf(f.t, func() bool {
		desc.Reset()
		actualSelectors := f.pw.ExtraSelectors(keyForManifest(mn))
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
	state := f.store.LockMutableStateForTesting()
	state.RemoveManifestTarget(mn)
	f.store.UnlockMutableState()

	f.ms.OnChange(f.ctx, f.store, store.LegacyChangeSummary())
	f.waitForExtraSelectors(mn)
}

type pwFixture struct {
	*tempdir.TempDirFixture
	t             *testing.T
	kClient       *k8s.FakeK8sClient
	ms            *ManifestSubscriber
	pw            *PodWatcher
	ctx           context.Context
	cancel        func()
	store         *store.Store
	pods          []*v1alpha1.Pod
	manifestNames []model.ManifestName
	mu            sync.Mutex
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
	case PodChangeAction:
		pw.pods = append(pw.pods, a.Pod)
		pw.manifestNames = append(pw.manifestNames, a.ManifestName)
	case store.PanicAction:
		pw.t.Fatalf("Store received PanicAction: %v", a.Err)
	default:
		pw.t.Fatalf("Unexpected action type: %T", action)
	}
}

func newPWFixture(t *testing.T) *pwFixture {
	kClient := k8s.NewFakeK8sClient()

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	of := k8s.ProvideOwnerFetcher(ctx, kClient)
	pw := NewPodWatcher(kClient, of)

	ms := NewManifestSubscriber(k8s.DefaultNamespace)

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
	key := keyForManifest(mn)
	require.Eventuallyf(f.t, func() bool {
		return f.pw.HasUIDWatch(key, entity.UID())
	}, stdTimeout, 20*time.Millisecond, "Watch for manifest[%s] on entity[%s] not setup", mn, entity.Name())
}

func (f *pwFixture) waitForPodActionCount(count int) {
	f.t.Helper()
	require.Eventuallyf(f.t, func() bool {
		f.mu.Lock()
		defer f.mu.Unlock()
		return len(f.pods) >= count
	}, stdTimeout, 20*time.Millisecond, "Timeout waiting for %d pod actions", count)
}

func (f *pwFixture) assertObservedPods(pods ...*corev1.Pod) {
	f.t.Helper()
	if len(pods) == 0 {
		// since this waits on async actions, asserting on no pods is
		// not reliable as it's the default state so too race-y
		f.t.Fatal("Must assert on at least one pod")
	}

	f.waitForPodActionCount(len(pods))
	var toCmp []*v1alpha1.Pod
	for _, p := range pods {
		toCmp = append(toCmp, k8sconv.Pod(f.ctx, p))
	}
	require.ElementsMatch(f.t, toCmp, f.pods)
}

func (f *pwFixture) assertObservedManifests(manifests ...model.ManifestName) {
	f.t.Helper()
	start := time.Now()
	for time.Since(start) < stdTimeout {
		if len(manifests) == len(f.manifestNames) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(f.t, manifests, f.manifestNames)
}

func (f *pwFixture) clearPods() {
	f.pods = nil
	f.manifestNames = nil
}
