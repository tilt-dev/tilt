package kubernetesdiscovery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/core/cluster"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

	f.k8sClient(*kd).UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID})
}

func TestPodDiscoveryAncestorMatch(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")
	dep, rs := f.buildK8sDeployment(ns, "dep")

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

	f.injectK8sObjects(*kd, dep, rs)

	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	pod := f.buildPod(ns, "pod", nil, rs)
	f.injectK8sObjects(*kd, pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: rs.UID})

	// Make sure the owner is filled in.
	f.MustGet(key, kd)
	assert.Equal(t, &v1alpha1.PodOwner{
		Name:              "dep-rs",
		APIVersion:        "apps/v1",
		Kind:              "ReplicaSet",
		CreationTimestamp: rs.CreationTimestamp,
	}, kd.Status.Pods[0].Owner)

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

	_, rs := f.buildK8sDeployment(ns, "dep")
	pod := f.buildPod(ns, "pod", nil, rs)
	// pod is deployed before it or its ancestors are ever referenced by spec
	f.injectK8sObjects(*kd, pod)

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

	knownDep, knownRS := f.buildK8sDeployment(ns, "known")

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
	f.injectK8sObjects(*kd, knownDep, knownRS)

	f.Create(kd)
	f.requireMonitorStarted(key)
	// we should not have observed any pods yet
	f.requireObservedPods(key, nil)

	pod1 := f.buildPod(ns, "pod1", labels.Set{"k1": "v1", "k2": "v2", "other": "other1"}, nil)
	pod2 := f.buildPod(ns, "pod2", labels.Set{"k1": "v1", "other": "other2"}, nil)
	_, unknownRS := f.buildK8sDeployment(ns, "unknown")
	pod3 := f.buildPod(ns, "pod3", labels.Set{"k3": "v3"}, unknownRS)
	pod4 := f.buildPod(ns, "pod4", labels.Set{"k3": "v3"}, knownRS)
	f.injectK8sObjects(*kd, pod1, pod2, pod3, pod4)

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

	sharedDep, sharedRS := f.buildK8sDeployment(ns, "known")
	preExistingPod := f.buildPod(ns, "preexisting", nil, sharedRS)

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
	f.injectK8sObjects(*kd1, preExistingPod, sharedDep, sharedRS)
	f.Create(kd1)

	kd2Dep, kd2RS := f.buildK8sDeployment(ns, "kd2only")

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
	f.injectK8sObjects(*kd2, kd2Dep, kd2RS)
	f.Create(kd2)

	for _, k := range []types.NamespacedName{key1, key2} {
		f.requireMonitorStarted(k)
		// initially, both should have seen the pre-existing pod since they both watch the same RS
		f.requireObservedPods(k, ancestorMap{preExistingPod.UID: sharedRS.UID})
	}

	// pod1 matches on labels for both kd/kd2 and doesn't have any associated Deployment
	pod1 := f.buildPod(ns, "pod1", labels.Set{"k": "v"}, nil)

	// pod2 is another pod for the known, shared RS
	pod2 := f.buildPod(ns, "pod2", nil, nil)

	// pod3 is for the replicaset only known by KD2 but has labels that match KD1 as well
	pod3 := f.buildPod(ns, "pod3", labels.Set{"k": "v"}, kd2RS)

	f.injectK8sObjects(*kd1, pod1, pod2, pod3)

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

func TestReconcileCreatesPodLogStream(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")
	pod1 := f.buildPod(ns, "pod1", nil, nil)
	pod2 := f.buildPod(ns, "pod2", nil, nil)

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	sinceTime := apis.NewTime(time.Now())
	podLogStreamTemplateSpec := &v1alpha1.PodLogStreamTemplateSpec{
		SinceTime: &sinceTime,
		IgnoreContainers: []string{
			string(container.IstioInitContainerName),
			string(container.IstioSidecarContainerName),
		},
	}

	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod1.UID),
					Namespace: pod1.Namespace,
					Name:      pod1.Name,
				},
				{
					UID:       string(pod2.UID),
					Namespace: pod2.Namespace,
					Name:      pod2.Name,
				},
			},
			PodLogStreamTemplateSpec: podLogStreamTemplateSpec,
		},
	}

	f.injectK8sObjects(*kd, pod1, pod2)

	f.Create(kd)
	// make sure the pods have been seen so that it knows what to create resources for
	f.requireObservedPods(key, ancestorMap{pod1.UID: pod1.UID, pod2.UID: pod2.UID})

	// in reality, once the pods are observed, a status update is triggered, which would
	// result in a reconcile; but the test is not running under the manager, so an update
	// doesn't implicitly trigger a reconcile and we have to manually do it
	f.MustReconcile(key)

	var podLogStreams v1alpha1.PodLogStreamList
	f.List(&podLogStreams)
	require.Equal(t, 2, len(podLogStreams.Items), "Incorrect number of PodLogStream objects")

	sort.Slice(podLogStreams.Items, func(i, j int) bool {
		return podLogStreams.Items[i].Spec.Pod < podLogStreams.Items[j].Spec.Pod
	})

	assert.Equal(t, "pod1", podLogStreams.Items[0].Spec.Pod)
	assert.Equal(t, "pod2", podLogStreams.Items[1].Spec.Pod)

	for _, pls := range podLogStreams.Items {
		assert.Equal(t, ns.String(), pls.Spec.Namespace)

		timecmp.AssertTimeEqual(t, sinceTime, pls.Spec.SinceTime)

		assert.ElementsMatch(t,
			[]string{container.IstioInitContainerName.String(), container.IstioSidecarContainerName.String()},
			pls.Spec.IgnoreContainers)

		assert.Empty(t, pls.Spec.OnlyContainers)
	}

	// simulate a pod delete and ensure that after it's observed + reconciled, the PLS is also deleted
	f.k8sClient(*kd).EmitPodDelete(pod1)
	f.requireObservedPods(key, ancestorMap{pod2.UID: pod2.UID})
	f.MustReconcile(key)
	f.List(&podLogStreams)
	require.Equal(t, 1, len(podLogStreams.Items), "Incorrect number of PodLogStream objects")
	assert.Equal(t, "pod2", podLogStreams.Items[0].Spec.Pod)

	// simulate the PodLogStream being deleted by an external force - chaos!
	f.Delete(&podLogStreams.Items[0])
	f.List(&podLogStreams)
	assert.Empty(t, podLogStreams.Items)
	// similar to before, in reality, the reconciler watches the objects it owns, so the manager would
	// normally call reconcile automatically, but for the test we have to manually simulate it
	f.MustReconcile(key)
	f.List(&podLogStreams)
	require.Equal(t, 1, len(podLogStreams.Items), "Incorrect number of PodLogStream objects")
	assert.Equal(t, "pod2", podLogStreams.Items[0].Spec.Pod)
}

func TestKubernetesDiscoveryIndexing(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Cluster: "my-cluster",
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
		},
	}

	// fixture will automatically create a cluster object
	f.Create(kd)

	reqs := f.r.indexer.Enqueue(&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "my-cluster"}})
	assert.ElementsMatch(t, []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: "some-ns", Name: "kd"}},
	}, reqs)
}

func TestKubernetesDiscoveryClusterError(t *testing.T) {
	f := newFixture(t)

	pod := f.buildPod("pod-ns", "pod", nil, nil)

	key := types.NamespacedName{Namespace: "some-ns", Name: "kd"}
	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Cluster: "my-cluster",
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
		},
	}

	// cannot use normal fixture create flow because we want to intentionally
	// set things up in a bad state
	f.ensureCluster(*kd)
	f.clients.SetClusterError(clusterNN(*kd), errors.New("oh no"))
	require.NoError(t, f.Client.Create(f.Context(), kd), "Could not create KubernetesDiscovery")

	// TODO(milas): we need to add an Error field to KubernetesDiscoveryStatus
	// 	to report these problems
	_, err := f.Reconcile(key)
	require.EqualError(t, err, `failed to reconcile some-ns:kd: cluster "my-cluster" connection error: oh no`)
}

type fixture struct {
	*fake.ControllerFixture
	t       *testing.T
	clients *cluster.FakeClientCache
	r       *Reconciler
	ctx     context.Context
	store   *store.TestingStore
}

func newFixture(t *testing.T) *fixture {
	clients := cluster.NewFakeClientCache(nil)

	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)

	st := store.NewTestingStore()

	rd := NewContainerRestartDetector()
	cfb := fake.NewControllerFixtureBuilder(t)
	pw := NewReconciler(cfb.Client, cfb.Scheme(), clients, rd, st)

	ret := &fixture{
		ControllerFixture: cfb.Build(pw),
		clients:           clients,
		r:                 pw,
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

// buildK8sDeployment creates fake Deployment + associated ReplicaSet objects.
func (f *fixture) buildK8sDeployment(namespace k8s.Namespace, name string) (*appsv1.Deployment, *appsv1.ReplicaSet) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			UID:               types.UID(name + "-uid"),
			Namespace:         namespace.String(),
			Name:              name,
			CreationTimestamp: apis.Now(),
		},
	}
	rsName := name + "-rs"
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			UID:               types.UID(rsName + "-uid"),
			Namespace:         namespace.String(),
			Name:              rsName,
			CreationTimestamp: apis.Now(),
			OwnerReferences:   []metav1.OwnerReference{k8s.RuntimeObjToOwnerRef(d)},
		},
	}

	return d, rs
}

// injectK8sObjects seeds objects in the fake K8s client for subsequent retrieval.
// This allow the reconciler to build object owner trees.
func (f *fixture) injectK8sObjects(kd v1alpha1.KubernetesDiscovery, objs ...runtime.Object) {
	f.t.Helper()
	kCli := f.k8sClient(kd)

	var k8sEntities []k8s.K8sEntity
	for _, obj := range objs {
		if pod, ok := obj.(*v1.Pod); ok {
			kCli.UpsertPod(pod)
			continue
		}

		k8sEntities = append(k8sEntities, k8s.NewK8sEntity(obj))
	}

	// inject these so that their metadata can be found later for owner reference matching
	kCli.Inject(k8sEntities...)
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
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
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

func clusterNN(kd v1alpha1.KubernetesDiscovery) types.NamespacedName {
	nn := types.NamespacedName{Namespace: kd.Namespace, Name: kd.Spec.Cluster}
	if nn.Name == "" {
		nn.Name = v1alpha1.ClusterNameDefault
	}
	return nn
}

func (f *fixture) k8sClient(kd v1alpha1.KubernetesDiscovery) *k8s.FakeK8sClient {
	f.t.Helper()

	clusterNN := clusterNN(kd)
	kCli, err := f.clients.GetK8sClient(clusterNN)
	if errors.Is(err, cluster.NotFoundError) {
		fakeCli := k8s.NewFakeK8sClient(f.t)
		f.t.Cleanup(fakeCli.TearDown)
		f.clients.AddK8sClient(clusterNN, fakeCli)
		kCli, err = f.clients.GetK8sClient(clusterNN)
	}
	require.NoError(f.t, err, "Couldn't get K8s client for cluster %s", clusterNN)
	require.IsType(f.t, &k8s.FakeK8sClient{}, kCli)
	return kCli.(*k8s.FakeK8sClient)
}

func (f *fixture) ensureCluster(kd v1alpha1.KubernetesDiscovery) {
	f.t.Helper()

	// seed the k8s client if it doesn't already exist
	f.k8sClient(kd)

	nn := clusterNN(kd)

	f.ControllerFixture.Upsert(&v1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nn.Namespace,
			Name:      nn.Name,
		},
		Spec: v1alpha1.ClusterSpec{
			Connection: &v1alpha1.ClusterConnection{
				Kubernetes: &v1alpha1.KubernetesClusterConnection{},
			},
		},
	})
}

func (f *fixture) Create(kd *v1alpha1.KubernetesDiscovery) controllerruntime.Result {
	f.t.Helper()
	kd.Default()
	f.ensureCluster(*kd)
	return f.ControllerFixture.Create(kd)
}
