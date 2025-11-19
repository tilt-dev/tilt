package kubernetesdiscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/kubernetesdiscoverys"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

var (
	apiGVStr   = v1alpha1.SchemeGroupVersion.String()
	apiKind    = "KubernetesDiscovery"
	apiType    = metav1.TypeMeta{Kind: apiKind, APIVersion: apiGVStr}
	clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")
)

type namespaceSet map[string]bool
type watcherSet map[watcherID]bool

// watcherID is to disambiguate between K8s object keys and tilt-apiserver KubernetesDiscovery object keys.
type watcherID types.NamespacedName

func (w watcherID) String() string {
	return types.NamespacedName(w).String()
}

type Reconciler struct {
	clients    *cluster.ClientManager
	st         store.Dispatcher
	indexer    *indexer.Indexer
	ctrlClient ctrlclient.Client
	requeuer   *indexer.Requeuer

	// restartDetector compares a previous version of status with the latest and emits log events
	// for any containers on the pod that restarted.
	restartDetector *ContainerRestartDetector

	// mu should be held throughout OnChange; helper methods used by it expect it to be held.
	// Any helper methods for the dispatch loop should claim the lock as needed.
	mu sync.Mutex

	// watchedNamespaces tracks the namespaces that are being observed for Pod events.
	//
	// For efficiency, a single watch is created for a given namespace and keys of watchers
	// are tracked; once there are no more watchers, cleanupAbandonedNamespaces will cancel
	// the watch.
	watchedNamespaces map[nsKey]nsWatch

	// watchers reflects the current state of the Reconciler namespace + UID watches.
	//
	// On reconcile, if the latest spec differs from what's tracked here, it will be acted upon.
	watchers map[watcherID]watcher

	// uidWatchers are the KubernetesDiscovery objects that have a watch ref for a particular K8s UID,
	// and so will receive events for changes to it (in addition to specs that match based on Pod labels).
	uidWatchers map[uidKey]watcherSet

	// knownDescendentPodUIDs maps the UID of Kubernetes resources to the UIDs of
	// all pods that they own (transitively).
	//
	// For example, a Deployment UID might contain a set of N pod UIDs.
	knownDescendentPodUIDs map[uidKey]k8s.UIDSet

	// knownPods is an index of all the known pods and associated Tilt-derived metadata, by UID.
	knownPods             map[uidKey]*v1.Pod
	knownPodOwnerCreation map[uidKey]metav1.Time

	// deletedPods is an index of pods that have been deleted from the cluster,
	// but are preserved for their termination status.
	//
	// Newer versions of Kubernetes have added 'ttl' fields that delete pods
	// after they terminate. We want tilt to hang onto these pods, even
	// if they're deleted from the cluster.
	//
	// If a Pod is in gcPods it MUST exist in known pods.
	deletedPods map[uidKey]bool
}

func (w *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesDiscovery{}).
		Owns(&v1alpha1.PodLogStream{}).
		Owns(&v1alpha1.PortForward{}).
		Watches(&v1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(w.indexer.Enqueue)).
		WatchesRawSource(w.requeuer)
	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, scheme *runtime.Scheme, clients cluster.ClientProvider, restartDetector *ContainerRestartDetector,
	st store.RStore) *Reconciler {
	return &Reconciler{
		ctrlClient:             ctrlClient,
		clients:                cluster.NewClientManager(clients),
		restartDetector:        restartDetector,
		requeuer:               indexer.NewRequeuer(),
		st:                     st,
		indexer:                indexer.NewIndexer(scheme, indexKubernetesDiscovery),
		watchedNamespaces:      make(map[nsKey]nsWatch),
		uidWatchers:            make(map[uidKey]watcherSet),
		watchers:               make(map[watcherID]watcher),
		knownDescendentPodUIDs: make(map[uidKey]k8s.UIDSet),
		knownPods:              make(map[uidKey]*v1.Pod),
		knownPodOwnerCreation:  make(map[uidKey]metav1.Time),
		deletedPods:            make(map[uidKey]bool),
	}
}

type watcher struct {
	// spec is the current version of the KubernetesDiscoverySpec being used for this watcher.
	//
	// It's used to simplify diffing logic and determine if action is needed.
	spec      v1alpha1.KubernetesDiscoverySpec
	startTime time.Time

	// extraSelectors are label selectors used to match pods that don't transitively match any known UID.
	extraSelectors []labels.Selector
	cluster        clusterKey
	errorReason    string
}

// nsWatch tracks the watchers for the given namespace and allows the watch to be canceled.
type nsWatch struct {
	watchers map[watcherID]bool
	cancel   context.CancelFunc
}

// Reconcile manages namespace watches for the modified KubernetesDiscovery object.
func (w *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := watcherID(request.NamespacedName)
	existing, hasExisting := w.watchers[key]

	kd, err := w.getKubernetesDiscovery(ctx, key)
	w.indexer.OnReconcile(request.NamespacedName, kd)
	if err != nil {
		return ctrl.Result{}, err
	}

	if kd == nil || !kd.ObjectMeta.DeletionTimestamp.IsZero() {
		// spec was deleted - just clean up any watches and we're done
		if hasExisting {
			w.teardown(key)
			w.cleanupAbandonedNamespaces()
		}

		if err := w.manageOwnedObjects(ctx, request.NamespacedName, nil); err != nil {
			return ctrl.Result{}, err
		}

		w.st.Dispatch(kubernetesdiscoverys.NewKubernetesDiscoveryDeleteAction(request.NamespacedName.Name))
		return ctrl.Result{}, nil
	}

	ctx = store.MustObjectLogHandler(ctx, w.st, kd)

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	w.st.Dispatch(kubernetesdiscoverys.NewKubernetesDiscoveryUpsertAction(kd))

	cluster, err := w.getCluster(ctx, kd)
	if err != nil {
		return ctrl.Result{}, err
	}
	needsRefresh := w.clients.Refresh(kd, cluster)

	if !hasExisting || needsRefresh || !apicmp.DeepEqual(existing.spec, kd.Spec) {
		w.addOrReplace(ctx, key, kd, cluster)
	}

	kd, err = w.maybeUpdateObjectStatus(ctx, kd, key)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := w.manageOwnedObjects(ctx, request.NamespacedName, kd); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (w *Reconciler) getCluster(ctx context.Context, kd *v1alpha1.KubernetesDiscovery) (*v1alpha1.Cluster, error) {
	if kd.Spec.Cluster == "" {
		return nil, errors.New("cluster name is empty")
	}

	clusterNN := types.NamespacedName{Namespace: kd.Namespace, Name: kd.Spec.Cluster}
	var cluster v1alpha1.Cluster
	err := w.ctrlClient.Get(ctx, clusterNN, &cluster)
	if err != nil {
		return nil, err
	}
	return &cluster, nil
}

// getKubernetesDiscovery returns the KubernetesDiscovery object for the given key.
//
// If the API returns NotFound, nil will be returned for both the KubernetesDiscovery object AND error to simplify
// error-handling for callers. All other errors will result in an a wrapped error being passed along.
func (w *Reconciler) getKubernetesDiscovery(ctx context.Context, key watcherID) (*v1alpha1.KubernetesDiscovery, error) {
	nn := types.NamespacedName(key)
	var kd v1alpha1.KubernetesDiscovery
	if err := w.ctrlClient.Get(ctx, nn, &kd); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get KubernetesDiscovery status for %q: %w", key, err)
	}
	return &kd, nil
}

func (w *Reconciler) addOrReplace(ctx context.Context, watcherKey watcherID, kd *store.KubernetesDiscovery, cluster *v1alpha1.Cluster) {
	if _, ok := w.watchers[watcherKey]; ok {
		// if a watcher already exists, just tear it down and we'll set it up from scratch so that
		// we don't have to diff a bunch of different pieces
		w.teardown(watcherKey)
	}

	defer func() {
		// ensure that any namespaces of which this was the last watcher have their watch stopped
		w.cleanupAbandonedNamespaces()
	}()

	var extraSelectors []labels.Selector
	for _, s := range kd.Spec.ExtraSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&s)
		if err != nil {
			w.watchers[watcherKey] = watcher{
				spec:        *kd.Spec.DeepCopy(),
				cluster:     newClusterKey(cluster),
				errorReason: fmt.Sprintf("invalid label selectors: %v", err),
			}
			return
		}
		extraSelectors = append(extraSelectors, selector)
	}

	newWatcher := watcher{
		spec:           *kd.Spec.DeepCopy(),
		extraSelectors: extraSelectors,
		cluster:        newClusterKey(cluster),
	}

	kCli, err := w.clients.GetK8sClient(kd, cluster)
	if err != nil {
		newWatcher.errorReason = "ClusterUnavailable"
	} else {
		currentNamespaces, currentUIDs := namespacesAndUIDsFromSpec(kd.Spec.Watches)
		for namespace := range currentNamespaces {
			nsKey := newNsKey(cluster, namespace)
			err := w.setupNamespaceWatch(ctx, nsKey, watcherKey, kCli)
			if err != nil {
				newWatcher.errorReason = err.Error()
				break
			}
		}

		if newWatcher.errorReason == "" {
			for watchUID := range currentUIDs {
				w.setupUIDWatch(ctx, newUIDKey(cluster, watchUID), watcherKey)
			}

			newWatcher.startTime = time.Now()
		}
	}

	w.watchers[watcherKey] = newWatcher
}

// teardown removes the watcher from all namespace + UIDs it was watching.
//
// By design, teardown does NOT clean up any watches for namespaces that no longer have any active watchers.
// This is done by calling cleanupAbandonedNamespaces explicitly, which allows addOrReplace to have simpler logic
// by always calling teardown on a resource, then treating it as "new" and only cleaning up after it has (re-)added
// the watches without needlessly removing + recreating the lower-level namespace watch.
func (w *Reconciler) teardown(watcherKey watcherID) {
	watcher := w.watchers[watcherKey]
	namespaces, uids := namespacesAndUIDsFromSpec(watcher.spec.Watches)
	for nsKey, nsWatch := range w.watchedNamespaces {
		if namespaces[nsKey.namespace] {
			delete(nsWatch.watchers, watcherKey)
		}
	}

	for uidKey, watchers := range w.uidWatchers {
		if uids[uidKey.uid] {
			delete(watchers, watcherKey)
		}
	}

	delete(w.watchers, watcherKey)
}

// cleanupAbandonedNamespaces removes the watch on any namespaces that no longer have any active watchers.
//
// mu must be held by caller.
//
// See watchedNamespaces for more details (for efficiency, we don't want duplicative namespace watches).
func (w *Reconciler) cleanupAbandonedNamespaces() {
	for nsKey, watcher := range w.watchedNamespaces {
		if len(watcher.watchers) == 0 {
			watcher.cancel()
			delete(w.watchedNamespaces, nsKey)
		}
	}
}

// setupNamespaceWatch creates a namespace watch if necessary and adds a key to the list of watchers for it.
//
// mu must be held by caller.
//
// It is idempotent:
//   - If no watch for the namespace exists, it is created and the given key is the sole watcher
//   - If a watch for the namespace exists but the given key is not in the watcher list, it is added
//   - If a watch for the namespace exists and the given key is already in the watcher list, it no-ops
//
// This ensures it can be safely called by reconcile on each invocation for any namespace that the watcher cares about.
// Additionally, for efficiency, duplicative watches on the same namespace will not be created; see watchedNamespaces
// for more details.
func (w *Reconciler) setupNamespaceWatch(ctx context.Context, nsKey nsKey, watcherKey watcherID, kCli k8s.Client) error {
	if watcher, ok := w.watchedNamespaces[nsKey]; ok {
		// already watching this namespace -- just add this watcher to the list for cleanup tracking
		watcher.watchers[watcherKey] = true
		return nil
	}

	ns := nsKey.namespace
	ch, err := kCli.WatchPods(ctx, k8s.Namespace(ns))
	if err != nil {
		return errors.Wrapf(err, "Error watching pods. Are you connected to kubernetes?\nTry running `kubectl get pods -n %q`", ns)
	}

	ctx, cancel := context.WithCancel(ctx)
	w.watchedNamespaces[nsKey] = nsWatch{
		watchers: map[watcherID]bool{watcherKey: true},
		cancel:   cancel,
	}

	go w.dispatchPodChangesLoop(ctx, nsKey, kCli.OwnerFetcher(), ch)
	return nil
}

// setupUIDWatch registers a watcher to receive updates for any Pods transitively owned by this UID (or that exactly
// match this UID).
//
// mu must be held by caller.
func (w *Reconciler) setupUIDWatch(_ context.Context, uidKey uidKey, watcherID watcherID) {
	if w.uidWatchers[uidKey][watcherID] {
		return
	}

	// add this key as a watcher for the UID
	uidWatchers, ok := w.uidWatchers[uidKey]
	if !ok {
		uidWatchers = make(watcherSet)
		w.uidWatchers[uidKey] = uidWatchers
	}
	uidWatchers[watcherID] = true
}

// updateStatus builds the latest status for the given KubernetesDiscovery spec
// key and persists it. Should only be called in the main reconciler thread.
//
// If the status has not changed since the last status update performed (by the
// Reconciler), it will be skipped.
//
// Returns the latest object on success.
func (w *Reconciler) maybeUpdateObjectStatus(ctx context.Context, kd *v1alpha1.KubernetesDiscovery, watcherID watcherID) (*v1alpha1.KubernetesDiscovery, error) {
	watcher := w.watchers[watcherID]
	status := w.buildStatus(ctx, watcher)
	if apicmp.DeepEqual(kd.Status, status) {
		// the status hasn't changed - avoid a spurious update
		return kd, nil
	}

	oldStatus := kd.Status
	oldError := w.statusError(kd.Status)

	update := kd.DeepCopy()
	update.Status = status
	err := w.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return nil, err
	}

	newError := w.statusError(update.Status)
	if newError != "" && oldError != newError {
		logger.Get(ctx).Errorf("kubernetesdiscovery %s: %s", update.Name, newError)
	}

	w.restartDetector.Detect(w.st, oldStatus, update)
	return update, nil
}

func (w *Reconciler) statusError(status v1alpha1.KubernetesDiscoveryStatus) string {
	if status.Waiting != nil {
		return status.Waiting.Reason
	}
	return ""
}

// buildStatus creates the current state for the given KubernetesDiscovery object key.
//
// mu must be held by caller.
func (w *Reconciler) buildStatus(ctx context.Context, watcher watcher) v1alpha1.KubernetesDiscoveryStatus {
	if watcher.errorReason != "" {
		return v1alpha1.KubernetesDiscoveryStatus{
			Waiting: &v1alpha1.KubernetesDiscoveryStateWaiting{
				Reason: watcher.errorReason,
			},
		}
	}

	seenPodUIDs := k8s.NewUIDSet()
	var pods []v1alpha1.Pod
	maybeTrackPod := func(pod *v1.Pod, ancestorUID types.UID) {
		if pod == nil || seenPodUIDs.Contains(pod.UID) {
			return
		}
		seenPodUIDs.Add(pod.UID)
		podObj := *k8sconv.Pod(ctx, pod, ancestorUID)
		if podObj.Owner != nil {
			podKey := uidKey{cluster: watcher.cluster, uid: pod.UID}
			podObj.Owner.CreationTimestamp = w.knownPodOwnerCreation[podKey]
		}
		pods = append(pods, podObj)
	}

	for i := range watcher.spec.Watches {
		watchUID := types.UID(watcher.spec.Watches[i].UID)
		if watchUID == "" || seenPodUIDs.Contains(watchUID) {
			continue
		}
		// UID could either refer directly to a Pod OR its ancestor (e.g. Deployment)
		watchedObjKey := uidKey{cluster: watcher.cluster, uid: watchUID}
		maybeTrackPod(w.knownPods[watchedObjKey], watchUID)
		for podUID := range w.knownDescendentPodUIDs[watchedObjKey] {
			podKey := uidKey{cluster: watcher.cluster, uid: podUID}
			maybeTrackPod(w.knownPods[podKey], watchUID)
		}
	}

	// TODO(milas): we should only match against Pods in namespaces referenced by the WatchRefs for this spec
	if len(watcher.spec.ExtraSelectors) != 0 {
		for podKey, pod := range w.knownPods {
			if podKey.cluster != watcher.cluster || seenPodUIDs.Contains(podKey.uid) {
				// ignore pods that are for other clusters or that we've already seen
				continue
			}
			podLabels := labels.Set(pod.Labels)
			for _, selector := range watcher.extraSelectors {
				if selector.Matches(podLabels) {
					maybeTrackPod(pod, "")
					break
				}
			}
		}
	}

	pods = w.maybeLetGoOfDeletedPods(pods, watcher.cluster)

	startTime := apis.NewMicroTime(watcher.startTime)
	return v1alpha1.KubernetesDiscoveryStatus{
		MonitorStartTime: startTime,
		Pods:             pods,
		Running: &v1alpha1.KubernetesDiscoveryStateRunning{
			StartTime: startTime,
		},
	}
}

// If a pod was deleted from the cluster, check to make sure if we
// should delete it from our local store.
func (w *Reconciler) maybeLetGoOfDeletedPods(pods []v1alpha1.Pod, clusterKey clusterKey) []v1alpha1.Pod {
	allDeleted := true
	someDeleted := false
	for _, pod := range pods {
		key := uidKey{cluster: clusterKey, uid: types.UID(pod.UID)}
		isDeleted := w.deletedPods[key]
		if isDeleted {
			someDeleted = true
		} else {
			allDeleted = false
		}
	}

	if allDeleted || !someDeleted {
		return pods
	}

	result := make([]v1alpha1.Pod, 0, len(pods))
	for _, pod := range pods {
		key := uidKey{cluster: clusterKey, uid: types.UID(pod.UID)}
		isDeleted := w.deletedPods[key]
		if isDeleted {
			delete(w.knownPods, key)
			delete(w.knownPodOwnerCreation, key)
			delete(w.deletedPods, key)
		} else {
			result = append(result, pod)
		}
	}
	return result
}

func (w *Reconciler) upsertPod(cluster clusterKey, pod *v1.Pod) {
	w.mu.Lock()
	defer w.mu.Unlock()
	podKey := uidKey{cluster: cluster, uid: pod.UID}
	w.knownPods[podKey] = pod
}

// triageResult is a KubernetesDiscovery key and the UID (if any) of the watch ref that matched the Pod event.
type triageResult struct {
	watcherID   watcherID
	ancestorUID types.UID
}

// triagePodTree checks to see if this Pod corresponds to any of the KubernetesDiscovery objects.
//
// Currently, we do this by comparing the Pod UID and its owner UIDs against watched UIDs from
// KubernetesDiscovery specs. More than one KubernetesDiscovery object can watch the same UID
// and each will receive an event. (Note that currently, ManifestSubscriber uniquely assigns
// UIDs to prevent more than one manifest from watching the same UID, but at an API level, it's
// possible.)
//
// Additionally, the Pod's labels will be evaluated against any extra selectors from the specs
// and reporting for any specs that it matches. (If a watcher already matched explicitly via
// transitive UID ownership, it will not be evaluated for label match.)
//
// Even if the Pod doesn't match any KubernetesDiscovery spec, it's still kept in local state,
// so we can match it later if a KubernetesDiscovery spec is modified to match it; this is actually
// extremely common because new Pods are typically observed by Reconciler _before_ the respective
// KubernetesDiscovery spec update propagates.
func (w *Reconciler) triagePodTree(nsKey nsKey, pod *v1.Pod, objTree k8s.ObjectRefTree) []triageResult {
	podUID := pod.UID
	if len(objTree.Owners) > 0 {
		podKey := uidKey{cluster: nsKey.cluster, uid: podUID}
		w.knownPodOwnerCreation[podKey] = objTree.Owners[0].CreationTimestamp
	}

	// Set up the descendent pod UID index
	for _, ownerUID := range objTree.UIDs() {
		if podUID == ownerUID {
			continue
		}

		ownerKey := uidKey{cluster: nsKey.cluster, uid: ownerUID}
		set, ok := w.knownDescendentPodUIDs[ownerKey]
		if !ok {
			set = k8s.NewUIDSet()
			w.knownDescendentPodUIDs[ownerKey] = set
		}
		set.Add(podUID)
	}

	seenWatchers := make(map[watcherID]bool)
	var results []triageResult

	// Find any watchers that have a ref to a UID in the object tree (i.e. the Pod itself or a transitive owner)
	for _, ownerUID := range objTree.UIDs() {
		ownerKey := uidKey{cluster: nsKey.cluster, uid: ownerUID}
		for watcherID := range w.uidWatchers[ownerKey] {
			if seenWatchers[watcherID] {
				// in practice, it's not really logical that a watcher would have more than one part of the
				// object tree watched, but since we already need to track seen watchers to skip duplicative
				// label matches, we might as well avoid it from becoming an issue
				// (also, if it does happen - the object tree should have consistent iteration order so a Pod
				//  will always match on a consistent ancestor UID, which avoids a spurious updates)
				continue
			}
			seenWatchers[watcherID] = true
			results = append(results, triageResult{watcherID: watcherID, ancestorUID: ownerUID})
		}
	}

	// NOTE(nick): This code might be totally obsolete now that we triage
	// pods by owner UID. It's meant to handle CRDs, but most CRDs should
	// set owner reference appropriately.
	podLabels := labels.Set(pod.ObjectMeta.GetLabels())
	for key, watcher := range w.watchers {
		if seenWatchers[key] {
			continue
		}
		for _, selector := range watcher.extraSelectors {
			if selector.Matches(podLabels) {
				seenWatchers[key] = true
				// there is no ancestorUID since this was a label match
				results = append(results, triageResult{watcherID: key, ancestorUID: ""})
				break
			}
		}
	}

	return results
}

func (w *Reconciler) handlePodChange(ctx context.Context, nsKey nsKey, ownerFetcher k8s.OwnerFetcher, pod *v1.Pod) {
	objTree, err := ownerFetcher.OwnerTreeOf(ctx, k8s.NewK8sEntity(pod))
	if err != nil {
		// In locked-down clusters, the user may not have access to certain types of resources
		// so it's normal for there to be errors. Ignore them.
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	triageResults := w.triagePodTree(nsKey, pod, objTree)
	for i := range triageResults {
		watcherID := triageResults[i].watcherID
		w.requeuer.Add(types.NamespacedName(watcherID))
	}
}

func (w *Reconciler) handlePodDelete(namespace k8s.Namespace, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var matchedPodKey uidKey
	var matchedPod *v1.Pod
	for podKey, pod := range w.knownPods {
		if pod.Namespace == namespace.String() && pod.Name == name {
			matchedPodKey = podKey
			matchedPod = pod
			break
		}
	}

	if matchedPodKey.uid == "" {
		// this pod wasn't known/tracked
		return
	}

	// If the pod is in a completed state when it was deleted, we may still needs
	// its status.  Hold onto it until we have more pods.
	phase := matchedPod.Status.Phase
	isCompleted := phase == v1.PodSucceeded || phase == v1.PodFailed
	if isCompleted {
		w.deletedPods[matchedPodKey] = true
	} else {
		delete(w.knownPods, matchedPodKey)
		delete(w.knownPodOwnerCreation, matchedPodKey)
		delete(w.deletedPods, matchedPodKey)
	}

	// because we don't know if any watchers matched on this Pod by label previously,
	// trigger an update on every watcher for the Pod's cluster, which will return
	// early if it didn't change
	for watcherID, watcher := range w.watchers {
		if watcher.cluster != matchedPodKey.cluster {
			continue
		}

		w.requeuer.Add(types.NamespacedName(watcherID))
	}
}

func (w *Reconciler) manageOwnedObjects(ctx context.Context, nn types.NamespacedName, kd *v1alpha1.KubernetesDiscovery) error {
	if err := w.manageOwnedPodLogStreams(ctx, nn, kd); err != nil {
		return err
	}

	if err := w.manageOwnedPortForwards(ctx, nn, kd); err != nil {
		return err
	}
	return nil
}

// Reconcile all the pod log streams owned by this KD. The KD may be nil if it's being deleted.
func (w *Reconciler) manageOwnedPodLogStreams(ctx context.Context, nn types.NamespacedName, kd *v1alpha1.KubernetesDiscovery) error {
	var managedPodLogStreams v1alpha1.PodLogStreamList
	err := indexer.ListOwnedBy(ctx, w.ctrlClient, &managedPodLogStreams, nn, apiType)
	if err != nil {
		return fmt.Errorf("failed to fetch managed PodLogStream objects for KubernetesDiscovery %s: %v",
			nn.Name, err)
	}
	plsByPod := make(map[types.NamespacedName]v1alpha1.PodLogStream)
	for _, pls := range managedPodLogStreams.Items {
		plsByPod[types.NamespacedName{
			Namespace: pls.Spec.Namespace,
			Name:      pls.Spec.Pod,
		}] = pls
	}

	var errs []error
	seenPods := make(map[types.NamespacedName]bool)
	if kd != nil {
		for _, pod := range kd.Status.Pods {
			podNN := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
			seenPods[podNN] = true
			if _, ok := plsByPod[podNN]; ok {
				// if the PLS gets modified after being created, just leave it as-is
				continue
			}

			if err := w.createPodLogStream(ctx, kd, pod); err != nil {
				errs = append(errs, fmt.Errorf("failed to create PodLogStream for Pod %s:%s for KubernetesDiscovery %s: %v",
					pod.Namespace, pod.Name, nn.Name, err))
			}
		}
	}

	for podKey, pls := range plsByPod {
		if !seenPods[podKey] {
			if err := w.ctrlClient.Delete(ctx, &pls); ctrlclient.IgnoreNotFound(err) != nil {
				errs = append(errs, fmt.Errorf("failed to delete PodLogStream %s for KubernetesDiscovery %s: %v",
					pls.Name, nn.Name, err))
			}
		}
	}

	return errorutil.NewAggregate(errs)
}

func (w *Reconciler) createPodLogStream(ctx context.Context, kd *v1alpha1.KubernetesDiscovery, pod v1alpha1.Pod) error {
	plsKey := types.NamespacedName{
		Namespace: kd.Namespace,
		Name:      fmt.Sprintf("%s-%s-%s", kd.Name, pod.Namespace, pod.Name),
	}

	manifest := kd.Annotations[v1alpha1.AnnotationManifest]
	spanID := string(k8sconv.SpanIDForPod(model.ManifestName(manifest), k8s.PodID(pod.Name)))

	plsTemplate := kd.Spec.PodLogStreamTemplateSpec

	// If there's no podlogtream template, create a default one.
	if plsTemplate == nil {
		plsTemplate = &v1alpha1.PodLogStreamTemplateSpec{}
	}

	// create PLS
	pls := v1alpha1.PodLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      plsKey.Name,
			Namespace: kd.Namespace,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: manifest,
				v1alpha1.AnnotationSpanID:   spanID,
			},
		},
		Spec: v1alpha1.PodLogStreamSpec{
			Pod:              pod.Name,
			Namespace:        pod.Namespace,
			SinceTime:        plsTemplate.SinceTime,
			IgnoreContainers: plsTemplate.IgnoreContainers,
			OnlyContainers:   plsTemplate.OnlyContainers,
		},
	}

	if err := controllerutil.SetControllerReference(kd, &pls, w.ctrlClient.Scheme()); err != nil {
		return err
	}

	if err := w.ctrlClient.Create(ctx, &pls); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}

	return nil
}

func (w *Reconciler) dispatchPodChangesLoop(ctx context.Context, nsKey nsKey, ownerFetcher k8s.OwnerFetcher,
	ch <-chan k8s.ObjectUpdate) {
	for {
		select {
		case obj, ok := <-ch:
			if !ok {
				// Log the closure and trigger reconciliation for all watchers in this namespace to recreate the watch.
				logger.Get(ctx).Infof("Pod watch channel closed for namespace %s in cluster %s. Triggering reconciliation to restart watch.",
					nsKey.namespace, nsKey.cluster)
				
				w.mu.Lock()
				if nsWatch, ok := w.watchedNamespaces[nsKey]; ok {
					for watcherID := range nsWatch.watchers {
						w.requeuer.Add(types.NamespacedName(watcherID))
					}
					delete(w.watchedNamespaces, nsKey)
				}
				w.mu.Unlock()
				return
			}

			pod, ok := obj.AsPod()
			if ok {
				w.upsertPod(nsKey.cluster, pod)
				go w.handlePodChange(ctx, nsKey, ownerFetcher, pod)
				continue
			}

			namespace, name, ok := obj.AsDeletedKey()
			if ok {
				go w.handlePodDelete(namespace, name)
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}

func namespacesAndUIDsFromSpec(watches []v1alpha1.KubernetesWatchRef) (namespaceSet, k8s.UIDSet) {
	seenNamespaces := make(namespaceSet)
	seenUIDs := k8s.NewUIDSet()

	for i := range watches {
		seenNamespaces[watches[i].Namespace] = true
		uid := types.UID(watches[i].UID)
		if uid != "" {
			// a watch ref might not have a UID:
			// 	* resources haven't been deployed yet
			//	* relies on extra label selectors from spec
			seenUIDs.Add(uid)
		}
	}

	return seenNamespaces, seenUIDs
}

// indexKubernetesDiscovery returns keys for all the objects we need to watch based on the spec.
func indexKubernetesDiscovery(obj ctrlclient.Object) []indexer.Key {
	var result []indexer.Key

	kd := obj.(*v1alpha1.KubernetesDiscovery)
	if kd != nil && kd.Spec.Cluster != "" {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{
				Namespace: kd.Namespace,
				Name:      kd.Spec.Cluster,
			},
			GVK: clusterGVK,
		})
	}

	return result
}
