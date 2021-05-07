package kubernetesdiscovery

import (
	"context"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/fake"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
)

const stdTimeout = time.Second

type ancestorMap map[types.UID]types.UID

func TestPodDiscoveryExactMatch(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
		},
	}

	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	f.kClient.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID})
}

func TestPodDiscoveryAncestorMatch(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")
	_, rs := f.simulateDeployment(ns, "dep")

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(rs.UID),
					Namespace: ns.String(),
					Name:      rs.Name,
				},
			},
		},
	}

	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	pod := f.buildPod(ns, "pod", nil, rs)
	f.kClient.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: rs.UID})

	// update the spec, changing the UID
	f.Get(key, kd)
	kd.Spec.Watches[0].UID = "unknown-uid"
	f.Update(kd)

	// no pods should be seen now
	f.requireObservedPods(key, nil)
}

func TestPodDiscoveryPreexisting(t *testing.T) {
	f := newFixture(t)
	ns := k8s.Namespace("ns")

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{Namespace: ns.String()},
			},
		},
	}
	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	_, rs := f.simulateDeployment(ns, "dep")
	pod := f.buildPod(ns, "pod", nil, rs)
	// pod is deployed before it or its ancestors are ever referenced by spec
	f.kClient.UpsertPod(pod)

	// typically, the reconciler will see the Pod event BEFORE any client is able to create
	// a spec that references it via ancestor UID and we still want those included on the status
	f.MustGet(key, kd)
	kd.Spec.Watches[0].UID = string(rs.UID)
	f.Update(kd)

	f.requireObservedPods(key, ancestorMap{pod.UID: rs.UID})
}

func TestPodDiscoveryLabelMatch(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")

	_, knownRS := f.simulateDeployment(ns, "known")

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{Namespace: ns.String()},
				{Namespace: ns.String(), UID: string(knownRS.UID)},
			},
			ExtraSelectors: []metav1.LabelSelector{
				*metav1.SetAsLabelSelector(labels.Set{"k1": "v1", "k2": "v2"}),
				*metav1.SetAsLabelSelector(labels.Set{"k3": "v3"}),
			},
		},
	}

	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	pod1 := f.buildPod(ns, "pod1", labels.Set{"k1": "v1", "k2": "v2", "other": "other1"}, nil)
	f.kClient.UpsertPod(pod1)

	pod2 := f.buildPod(ns, "pod2", labels.Set{"k1": "v1", "other": "other2"}, nil)
	f.kClient.UpsertPod(pod2)

	_, unknownRS := f.simulateDeployment(ns, "unknown")
	pod3 := f.buildPod(ns, "pod3", labels.Set{"k3": "v3"}, unknownRS)
	f.kClient.UpsertPod(pod3)

	pod4 := f.buildPod(ns, "pod4", labels.Set{"k3": "v3"}, knownRS)
	f.kClient.UpsertPod(pod4)

	// pod1 matches on labels and doesn't have any associated Deployment
	// pod2 does NOT match on labels - it must match ALL labels from a given set (it's missing k2:v2)
	// pod3 matches on labels and has a Deployment (that's NOT watched by this spec)
	// pod4 matches on a known ancestor AND labels but ancestor should take precedence
	f.requireObservedPods(key, ancestorMap{pod1.UID: "", pod3.UID: "", pod4.UID: knownRS.UID})

	// change the selectors around
	f.Get(key, kd)
	kd.Spec.ExtraSelectors[0] = *metav1.SetAsLabelSelector(labels.Set{"other": "other2"})
	kd.Spec.ExtraSelectors[1] = *metav1.SetAsLabelSelector(labels.Set{"k3": "v4"})
	f.Update(kd)

	// pod1 no longer matches
	// pod2 matches on labels
	// pod3 no longer matches
	// pod4 does NOT match on labels anymore should STILL be seen because it has a watched ancestor UID
	f.requireObservedPods(key, ancestorMap{pod2.UID: "", pod4.UID: knownRS.UID})
}

func TestPodDiscoveryDuplicates(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")

	_, sharedRS := f.simulateDeployment(ns, "known")
	preExistingPod := f.buildPod(ns, "preexisting", nil, sharedRS)
	f.kClient.UpsertPod(preExistingPod)

	key1 := types.NamespacedName{Namespace: "some-ns", Name: "kd1"}
	kd1 := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key1.Namespace, Name: key1.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{Namespace: ns.String()},
				{Namespace: ns.String(), UID: string(sharedRS.UID)},
			},
			ExtraSelectors: []metav1.LabelSelector{
				*metav1.SetAsLabelSelector(labels.Set{"k": "v"}),
			},
		},
	}
	f.Create(kd1)

	_, kd2RS := f.simulateDeployment(ns, "kd2only")

	key2 := types.NamespacedName{Namespace: "some-ns", Name: "kd2"}
	kd2 := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key2.Namespace, Name: key2.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{Namespace: ns.String()},
				{Namespace: ns.String(), UID: string(sharedRS.UID)},
				{Namespace: ns.String(), UID: string(kd2RS.UID)},
			},
			ExtraSelectors: []metav1.LabelSelector{
				*metav1.SetAsLabelSelector(labels.Set{"k": "v"}),
			},
		},
	}
	f.Create(kd2)

	for _, k := range []types.NamespacedName{key1, key2} {
		f.requireMonitorStarted(k)
		// initially, both should have seen the pre-existing pod since they both watch the same RS
		f.requireObservedPods(k, ancestorMap{preExistingPod.UID: sharedRS.UID})
	}

	// pod1 matches on labels for both kd/kd2 and doesn't have any associated Deployment
	pod1 := f.buildPod(ns, "pod1", labels.Set{"k": "v"}, nil)
	f.kClient.UpsertPod(pod1)

	// pod2 is another pod for the known, shared RS
	pod2 := f.buildPod(ns, "pod2", nil, nil)
	f.kClient.UpsertPod(pod2)

	// pod3 is for the replicaset only known by KD2 but has labels that match KD1 as well
	pod3 := f.buildPod(ns, "pod3", labels.Set{"k": "v"}, kd2RS)
	f.kClient.UpsertPod(pod3)

	f.requireObservedPods(key1, ancestorMap{
		preExistingPod.UID: sharedRS.UID,
		pod1.UID:           "", // label match
		pod3.UID:           "", // label match
	})

	f.requireObservedPods(key2, ancestorMap{
		preExistingPod.UID: sharedRS.UID,
		pod1.UID:           "",        // label match
		pod3.UID:           kd2RS.UID, // <-- unlike KD1, known RS!
	})
}

type fixture struct {
	*fake.ControllerFixture
	t       *testing.T
	kClient *k8s.FakeK8sClient
	pw      *Reconciler
	ctx     context.Context
	store   *store.TestingStore
}

func newFixture(t *testing.T) *fixture {
	kClient := k8s.NewFakeK8sClient(t)
	t.Cleanup(kClient.TearDown)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	st := store.NewTestingStore()

	of := k8s.ProvideOwnerFetcher(ctx, kClient)
	rd := NewContainerRestartDetector()
	pw := NewReconciler(kClient, of, rd, st)
	cf := fake.NewControllerFixture(t, pw)

	ret := &fixture{
		ControllerFixture: cf,
		kClient:           kClient,
		pw:                pw,
		ctx:               ctx,
		t:                 t,
		store:             st,
	}
	return ret
}

func (f *fixture) requireMonitorStarted(key types.NamespacedName) {
	f.t.Helper()
	var desc strings.Builder
	f.requireState(key, func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("object does not exist in apiserver")
			return false
		}
		if kd.Status.MonitorStartTime.IsZero() {
			desc.WriteString("monitor start time is zero")
			return false
		}
		return true
	}, "Monitor not started for key[%s]: %s", key, &desc)
}

func (f *fixture) requireObservedPods(key types.NamespacedName, expected ancestorMap) {
	f.t.Helper()

	if expected == nil {
		// just for easier comparison since nil != empty map
		expected = ancestorMap{}
	}

	var desc strings.Builder
	f.requireState(key, func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("object does not exist in apiserver")
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
	}, "Expected Pods were not observed for key[%s]: %s", key, &desc)
}

func (f *fixture) requireState(key types.NamespacedName, cond func(kd *v1alpha1.KubernetesDiscovery) bool, msg string, args ...interface{}) {
	f.t.Helper()
	require.Eventuallyf(f.t, func() bool {
		var kd v1alpha1.KubernetesDiscovery
		if !f.Get(key, &kd) {
			return cond(nil)
		}
		return cond(&kd)
	}, stdTimeout, 20*time.Millisecond, msg, args...)
}

// simulateDeployment creates a Deployment + associated ReplicaSet and injects them into the K8s client for subsequent
// retrieval (this allow the reconciler to build object owner trees).
func (f *fixture) simulateDeployment(namespace k8s.Namespace, name string) (*appsv1.Deployment, *appsv1.ReplicaSet) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(name + "-uid"),
			Namespace: namespace.String(),
			Name:      name,
		},
	}
	rsName := name + "-rs"
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			UID:             types.UID(rsName + "-uid"),
			Namespace:       namespace.String(),
			Name:            rsName,
			OwnerReferences: []metav1.OwnerReference{k8s.RuntimeObjToOwnerRef(d)},
		},
	}

	// inject these so that their metadata can be found later for owner reference matching
	f.kClient.Inject(k8s.NewK8sEntity(d), k8s.NewK8sEntity(rs))

	return d, rs
}

// buildPod makes a fake Pod object but does not simulate its deployment.
func (f *fixture) buildPod(namespace k8s.Namespace, name string, podLabels labels.Set, rs *appsv1.ReplicaSet) *v1.Pod {
	f.t.Helper()

	if podLabels == nil {
		podLabels = make(labels.Set)
	}

	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(name + "-uid"),
			Namespace: namespace.String(),
			Name:      name,
			Labels:    podLabels,
		},
	}

	if rs != nil {
		if rs.Namespace != p.Namespace {
			f.t.Fatalf("Pod (namespace=%s) cannot differ from ReplicaSet (namespace=%s)", p.Namespace, rs.Namespace)
		}
		p.OwnerReferences = []metav1.OwnerReference{k8s.RuntimeObjToOwnerRef(rs)}
	}

	return p
}
