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
	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

const stdTimeout = time.Second

type ancestorMap map[types.UID]types.UID
type podNameMap map[types.UID]string

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
	f.requireObservedPods(key, nil, nil)

	kCli := f.clients.MustK8sClient(clusterNN(*kd))
	kCli.UpsertPod(pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID}, nil)
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
	f.requireObservedPods(key, nil, nil)

	pod := f.buildPod(ns, "pod", nil, rs)
	f.injectK8sObjects(*kd, pod)

	f.requireObservedPods(key, ancestorMap{pod.UID: rs.UID}, nil)

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
	f.requireObservedPods(key, nil, nil)
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
	f.requireObservedPods(key, nil, nil)

	_, rs := f.buildK8sDeployment(ns, "dep")
	pod := f.buildPod(ns, "pod", nil, rs)
	// pod is deployed before it or its ancestors are ever referenced by spec
	f.injectK8sObjects(*kd, pod)

	// typically, the reconciler will see the Pod event BEFORE any client is able to create
	// a spec that references it via ancestor UID and we still want those included on the status
	f.MustGet(key, kd)
	kd.Spec.Watches[0].UID = string(rs.UID)
	f.Update(kd)

	f.requireObservedPods(key, ancestorMap{pod.UID: rs.UID}, nil)
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
	f.requireObservedPods(key, nil, nil)

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
	f.requireObservedPods(key, ancestorMap{pod1.UID: "", pod3.UID: "", pod4.UID: knownRS.UID}, nil)

	// change the selectors around
	f.Get(key, kd)
	kd.Spec.ExtraSelectors[0] = *metav1.SetAsLabelSelector(labels.Set{"other": "other2"})
	kd.Spec.ExtraSelectors[1] = *metav1.SetAsLabelSelector(labels.Set{"k3": "v4"})
	f.Update(kd)

	// pod1 no longer matches
	// pod2 matches on labels
	// pod3 no longer matches
	// pod4 does NOT match on labels anymore should STILL be seen because it has a watched ancestor UID
	f.requireObservedPods(key, ancestorMap{pod2.UID: "", pod4.UID: knownRS.UID}, nil)
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
		f.requireObservedPods(k, ancestorMap{preExistingPod.UID: sharedRS.UID}, nil)
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
	}, nil)

	f.requireObservedPods(key2, ancestorMap{
		preExistingPod.UID: sharedRS.UID,
		pod1.UID:           "",        // label match
		pod3.UID:           kd2RS.UID, // <-- unlike KD1, known RS!
	}, nil)
}

func TestReconcileManagesPodLogStream(t *testing.T) {
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
	f.requireObservedPods(key, ancestorMap{pod1.UID: pod1.UID, pod2.UID: pod2.UID}, nil)

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
	kCli := f.clients.MustK8sClient(clusterNN(*kd))
	kCli.EmitPodDelete(pod1)
	f.requireObservedPods(key, ancestorMap{pod2.UID: pod2.UID}, nil)
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

func TestReconcileManagesPortForward(t *testing.T) {
	f := newFixture(t)

	ns := k8s.Namespace("ns")
	pod := f.buildPod(ns, "pod", nil, nil)
	pod.Spec.Containers = []v1.Container{
		{
			Name: "container",
			Ports: []v1.ContainerPort{
				{ContainerPort: 7890, Protocol: v1.ProtocolTCP},
			},
		},
	}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{{Name: "container"}}

	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "some-ns",
			Name:      "ks",
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: "my-resource",
			},
		},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       string(pod.UID),
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
			},
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{{LocalPort: 1234}},
			},
		},
	}
	key := apis.Key(kd)

	f.injectK8sObjects(*kd, pod)

	f.Create(kd)
	// make sure the pods have been seen so that it knows what to create resources for
	f.requireObservedPods(key, ancestorMap{pod.UID: pod.UID}, nil)

	// in reality, once the pods are observed, a status update is triggered, which would
	// result in a reconcile; but the test is not running under the manager, so an update
	// doesn't implicitly trigger a reconcile and we have to manually do it
	f.MustReconcile(key)

	var portForwards v1alpha1.PortForwardList
	f.List(&portForwards)
	require.Len(t, portForwards.Items, 1)
	if assert.Len(t, portForwards.Items[0].Spec.Forwards, 1) {
		fwd := portForwards.Items[0].Spec.Forwards[0]
		assert.Equal(t, int32(1234), fwd.LocalPort)
		assert.Equal(t, int32(7890), fwd.ContainerPort)
	}

	f.AssertStdOutContains(
		`k8s_resource(name='my-resource', port_forwards='1234') currently maps localhost:1234 to port 7890 in your container.
A future version of Tilt will change this default and will map localhost:1234 to port 1234 in your container.
To keep your project working, change your Tiltfile to k8s_resource(name='my-resource', port_forwards='1234:7890')`)

	// simulate a pod delete and ensure that after it's observed + reconciled, the PF is also deleted
	kCli := f.clients.MustK8sClient(clusterNN(*kd))
	kCli.EmitPodDelete(pod)
	f.requireObservedPods(key, nil, nil)
	f.MustReconcile(key)
	f.List(&portForwards)
	require.Empty(t, portForwards.Items)
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

	reqs := f.r.indexer.Enqueue(context.Background(),
		&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "my-cluster"}})
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
	f.clients.EnsureK8sClusterError(f.ctx, clusterNN(*kd), errors.New("oh no"))
	require.NoError(t, f.Client.Create(f.Context(), kd), "Could not create KubernetesDiscovery")
	f.MustReconcile(key)
	f.MustGet(key, kd)

	require.NotNil(t, kd.Status.Waiting, "Waiting should be present")
	require.Equal(t, "ClusterUnavailable", kd.Status.Waiting.Reason)
	require.Zero(t, kd.Status.MonitorStartTime, "MonitorStartTime should not be populated")
	require.Nil(t, kd.Status.Running, "Running should not be populated")
}

func TestClusterChange(t *testing.T) {
	f := newFixture(t)

	kd1ClusterA := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{Namespace: "some-ns", Name: "kd1ClusterA"},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches: []v1alpha1.KubernetesWatchRef{
				{
					UID:       "pod1-uid",
					Namespace: "pod-ns",
				},
			},
			Cluster: "clusterA",
		},
	}
	kd2ClusterA := kd1ClusterA.DeepCopy()
	kd2ClusterA.Name = "kd2ClusterA"

	kd3ClusterB := kd1ClusterA.DeepCopy()
	kd3ClusterB.Name = "kd3ClusterB"
	kd3ClusterB.Spec.Cluster = "clusterB"

	// set up initial state
	for _, kd := range []*v1alpha1.KubernetesDiscovery{kd1ClusterA, kd2ClusterA, kd3ClusterB} {
		f.Create(kd)
		key := apis.Key(kd)
		f.requireMonitorStarted(key)
		// we should not have observed any pods yet
		f.requireObservedPods(key, nil, nil)
	}

	const pod1UID types.UID = "pod1-uid"
	const pod2UID types.UID = "pod2-uid"
	kCliClusterA := f.clients.MustK8sClient(clusterNN(*kd1ClusterA))

	pod1ClusterA := f.buildPod("pod-ns", "pod1ClusterA", nil, nil)
	pod1ClusterA.UID = pod1UID
	kCliClusterA.UpsertPod(pod1ClusterA)

	// this will be matched on later
	pod2ClusterA := f.buildPod("pod-ns", "pod2ClusterA", labels.Set{"foo": "bar"}, nil)
	pod2ClusterA.UID = pod2UID
	kCliClusterA.UpsertPod(pod2ClusterA)

	kCliClusterB := f.clients.MustK8sClient(clusterNN(*kd3ClusterB))
	// N.B. we intentionally use the same UIDs across both clusters!
	pod1ClusterB := pod1ClusterA.DeepCopy()
	pod1ClusterB.Name = "pod1ClusterB"
	kCliClusterB.UpsertPod(pod1ClusterB)

	pod2ClusterB := pod2ClusterA.DeepCopy()
	pod2ClusterB.Name = "pod2ClusterB"
	kCliClusterB.UpsertPod(pod2ClusterB)

	f.requireObservedPods(apis.Key(kd1ClusterA), ancestorMap{pod1UID: pod1UID}, podNameMap{pod1UID: "pod1ClusterA"})
	f.requireObservedPods(apis.Key(kd2ClusterA), ancestorMap{pod1UID: pod1UID}, podNameMap{pod1UID: "pod1ClusterA"})
	f.requireObservedPods(apis.Key(kd3ClusterB), ancestorMap{pod1UID: pod1UID}, podNameMap{pod1UID: "pod1ClusterB"})

	// create a NEW client for A
	kCliClusterA2 := k8s.NewFakeK8sClient(t)
	connectedAtA2 := f.clients.SetK8sClient(clusterNN(*kd1ClusterA), kCliClusterA2)

	// create copies of the old pods with slightly different names so we can
	// be sure we received the new ones
	pod1ClusterA2 := pod1ClusterA.DeepCopy()
	pod1ClusterA2.Name = "pod1ClusterA-2"
	kCliClusterA2.UpsertPod(pod1ClusterA2)

	pod2ClusterA2 := pod2ClusterA.DeepCopy()
	pod2ClusterA2.Name = "pod2ClusterA-2"
	kCliClusterA2.UpsertPod(pod2ClusterA2)

	// reconcile should succeed even though client is stale (and cannot be
	// refreshed due to lack of Cluster obj update, simulating a stale informer
	// cache) because the KD spec has not changed, so no watches will be (re)setup
	f.MustReconcile(apis.Key(kd1ClusterA))

	// on the other hand, no watches can be (re)setup, e.g. if spec changes
	f.MustGet(apis.Key(kd2ClusterA), kd2ClusterA)
	kd2ClusterA.Spec.ExtraSelectors = append(kd2ClusterA.Spec.ExtraSelectors,
		metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}})

	require.NoError(f.t, f.Client.Update(f.ctx, kd2ClusterA))
	f.MustReconcile(apis.Key(kd2ClusterA))
	f.MustGet(apis.Key(kd2ClusterA), kd2ClusterA)
	require.NotNil(t, kd2ClusterA.Status.Waiting, "kd2clusterA should be in waiting state")
	require.Equal(t, "ClusterUnavailable", kd2ClusterA.Status.Waiting.Reason)

	// cluster B can reconcile even if it has spec changes since it's using a
	// different cluster
	f.MustGet(apis.Key(kd3ClusterB), kd3ClusterB)
	kd3ClusterB.Spec.ExtraSelectors = append(kd3ClusterB.Spec.ExtraSelectors,
		metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}})
	f.Update(kd3ClusterB)
	f.MustReconcile(apis.Key(kd3ClusterB))

	// write the updated cluster obj to apiserver
	clusterA := f.getCluster(clusterNN(*kd1ClusterA))
	clusterA.Status.ConnectedAt = connectedAtA2.DeepCopy()
	require.NoError(f.t, f.Client.Status().Update(f.ctx, clusterA))

	// kd1 still only matches by UID but should see the Pod from the new cluster now
	f.MustReconcile(apis.Key(kd1ClusterA))
	f.requireObservedPods(apis.Key(kd1ClusterA), ancestorMap{pod1UID: pod1UID}, podNameMap{pod1UID: "pod1ClusterA-2"})

	// kd2 will now have 2 Pods - one by UID and one by label, but both from the new cluster
	// (note: because pod2 matches by label, there's no ancestor UID)
	f.MustReconcile(apis.Key(kd2ClusterA))
	f.requireObservedPods(apis.Key(kd2ClusterA),
		ancestorMap{pod1UID: pod1UID, pod2UID: ""},
		podNameMap{pod1UID: "pod1ClusterA-2", pod2UID: "pod2ClusterA-2"})

	// kd3 will now have 3 Pods - one by UID and one by label, both from its original cluster
	// (note: because pod2 matches by label, there's no ancestor UID)
	f.MustReconcile(apis.Key(kd3ClusterB))
	f.requireObservedPods(apis.Key(kd3ClusterB),
		ancestorMap{pod1UID: pod1UID, pod2UID: ""},
		podNameMap{pod1UID: "pod1ClusterB", pod2UID: "pod2ClusterB"})
}

func TestHangOntoDeletedPodsWhenNoSibling(t *testing.T) {
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
	f.requireObservedPods(key, nil, nil)

	podA := f.buildPod(ns, "pod-a", nil, rs)
	podA.Status.Phase = v1.PodSucceeded
	f.injectK8sObjects(*kd, podA)

	f.requireObservedPods(key, ancestorMap{podA.UID: rs.UID}, nil)

	kCli := f.clients.MustK8sClient(clusterNN(*kd))
	kCli.EmitPodDelete(podA)

	f.requireObservedPods(key, ancestorMap{podA.UID: rs.UID}, nil)

	podB := f.buildPod(ns, "pod-b", nil, rs)
	podB.Status.Phase = v1.PodRunning
	f.injectK8sObjects(*kd, podB)
	f.requireObservedPods(key, ancestorMap{podB.UID: rs.UID}, nil)
}

func TestNoHangOntoDeletedPodsWhenSiblingExists(t *testing.T) {
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
	f.requireObservedPods(key, nil, nil)

	podA := f.buildPod(ns, "pod-a", nil, rs)
	podA.Status.Phase = v1.PodSucceeded
	f.injectK8sObjects(*kd, podA)

	f.requireObservedPods(key, ancestorMap{podA.UID: rs.UID}, nil)

	podB := f.buildPod(ns, "pod-b", nil, rs)
	podB.Status.Phase = v1.PodRunning
	f.injectK8sObjects(*kd, podB)

	f.requireObservedPods(key, ancestorMap{podA.UID: rs.UID, podB.UID: rs.UID}, nil)

	kCli := f.clients.MustK8sClient(clusterNN(*kd))
	kCli.EmitPodDelete(podA)

	f.requireObservedPods(key, ancestorMap{podB.UID: rs.UID}, nil)
}

type fixture struct {
	*fake.ControllerFixture
	t       *testing.T
	r       *Reconciler
	ctx     context.Context
	clients *cluster.FakeClientProvider
}

func newFixture(t *testing.T) *fixture {
	rd := NewContainerRestartDetector()
	cfb := fake.NewControllerFixtureBuilder(t)
	clients := cluster.NewFakeClientProvider(t, cfb.Client)
	pw := NewReconciler(cfb.Client, cfb.Scheme(), clients, rd, cfb.Store)
	indexer.StartSourceForTesting(cfb.Context(), pw.requeuer, pw, nil)

	ret := &fixture{
		ControllerFixture: cfb.Build(pw),
		r:                 pw,
		ctx:               cfb.Context(),
		t:                 t,
		clients:           clients,
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

func (f *fixture) requireObservedPods(key types.NamespacedName, expectedAncestors ancestorMap, expectedNames podNameMap) {
	f.t.Helper()

	if expectedAncestors == nil {
		// just for easier comparison since nil != empty map
		expectedAncestors = ancestorMap{}
	}

	var desc strings.Builder
	f.requireState(key, func(kd *v1alpha1.KubernetesDiscovery) bool {
		desc.Reset()
		if kd == nil {
			desc.WriteString("object does not exist in apiserver")
			return false
		}
		actualAncestors := make(ancestorMap)
		actualNames := make(podNameMap)
		for _, p := range kd.Status.Pods {
			podUID := types.UID(p.UID)
			actualAncestors[podUID] = types.UID(p.AncestorUID)
			actualNames[podUID] = p.Name
		}

		if diff := cmp.Diff(expectedAncestors, actualAncestors); diff != "" {
			desc.WriteString("\n")
			desc.WriteString(diff)
			return false
		}

		// expectedNames are optional - we really care about UIDs but in some
		// cases it's useful to check names for multi-cluster cases
		if expectedNames != nil {
			if diff := cmp.Diff(expectedNames, actualNames); diff != "" {
				desc.WriteString("\n")
				desc.WriteString(diff)
				return false
			}
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
	f.clients.EnsureK8sCluster(f.ctx, clusterNN(kd))
	kCli := f.clients.MustK8sClient(clusterNN(kd))

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

func (f *fixture) getCluster(nn types.NamespacedName) *v1alpha1.Cluster {
	var c v1alpha1.Cluster
	f.MustGet(nn, &c)
	return &c
}

func (f *fixture) Create(kd *v1alpha1.KubernetesDiscovery) controllerruntime.Result {
	f.t.Helper()
	f.clients.EnsureK8sCluster(f.ctx, clusterNN(*kd))
	return f.ControllerFixture.Create(kd)
}
