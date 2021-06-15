package kubernetesdiscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/engine/k8swatch"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
)

type watcherSet map[watcherID]bool

// watcherID is to disambiguate between K8s object keys and tilt-apiserver KubernetesDiscovery object keys.
type watcherID types.NamespacedName

func (w watcherID) String() string {
	return types.NamespacedName(w).String()
}

type Reconciler struct {
	kCli         k8s.Client
	ownerFetcher k8s.OwnerFetcher
	store        Dispatcher
	ctrlClient   ctrlclient.Client

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
	watchedNamespaces map[k8s.Namespace]nsWatch

	// watchers reflects the current state of the Reconciler namespace + UID watches.
	//
	// On reconcile, if the latest spec differs from what's tracked here, it will be acted upon.
	watchers map[watcherID]watcher

	// uidWatchers are the KubernetesDiscovery objects that have a watch ref for a particular K8s UID,
	// and so will receive events for changes to it (in addition to specs that match based on Pod labels).
	uidWatchers map[types.UID]watcherSet

	// knownDescendentPodUIDs maps the UID of Kubernetes resources to the UIDs of
	// all pods that they own (transitively).
	//
	// For example, a Deployment UID might contain a set of N pod UIDs.
	knownDescendentPodUIDs map[types.UID]k8s.UIDSet

	// knownPods is an index of all the known pods and associated Tilt-derived metadata, by UID.
	knownPods map[types.UID]*v1.Pod
}

func (w *Reconciler) SetClient(client ctrlclient.Client) {
	w.ctrlClient = client
}

func (w *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesDiscovery{}).
		Complete(w)
}

func NewReconciler(kCli k8s.Client, ownerFetcher k8s.OwnerFetcher, restartDetector *ContainerRestartDetector,
	st store.RStore) *Reconciler {
	return &Reconciler{
		kCli:                   kCli,
		ownerFetcher:           ownerFetcher,
		restartDetector:        restartDetector,
		store:                  st,
		watchedNamespaces:      make(map[k8s.Namespace]nsWatch),
		uidWatchers:            make(map[types.UID]watcherSet),
		watchers:               make(map[watcherID]watcher),
		knownDescendentPodUIDs: make(map[types.UID]k8s.UIDSet),
		knownPods:              make(map[types.UID]*v1.Pod),
	}
}

type watcher struct {
	// spec is the current version of the KubernetesDiscoverySpec being used for this watcher.
	//
	// It's used to simplify diffing logic and determine if action is needed.
	spec      v1alpha1.KubernetesDiscoverySpec
	startTime time.Time
	// lastUpdate is the last version of the status that was persisted.
	//
	// It's used for diffing to detect container restarts as well as avoid spurious updates.
	lastUpdate v1alpha1.KubernetesDiscoveryStatus
	// extraSelectors are label selectors used to match pods that don't transitively match any known UID.
	extraSelectors []labels.Selector
}

// nsWatch tracks the watchers for the given namespace and allows the watch to be canceled.
type nsWatch struct {
	watchers map[watcherID]bool
	cancel   context.CancelFunc
}

// HasNamespaceWatch returns true if the key is a watcher for the given namespace.
//
// This is intended for use in tests.
func (w *Reconciler) HasNamespaceWatch(key types.NamespacedName, ns k8s.Namespace) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	nsWatch, ok := w.watchedNamespaces[ns]
	if !ok {
		return false
	}
	return nsWatch.watchers[watcherID(key)]
}

// HasUIDWatch returns true if the key is a watcher for the given K8s UID.
//
// This is intended for use in tests.
func (w *Reconciler) HasUIDWatch(key types.NamespacedName, uid types.UID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.uidWatchers[uid][watcherID(key)]
}

// ExtraSelectors returns the extra selectors for a given KubernetesDiscovery object.
//
// This is intended for use in tests.
func (w *Reconciler) ExtraSelectors(key types.NamespacedName) []labels.Selector {
	w.mu.Lock()
	defer w.mu.Unlock()
	var ret []labels.Selector
	for _, s := range w.watchers[watcherID(key)].extraSelectors {
		ret = append(ret, s.DeepCopySelector())
	}
	return ret
}

// Reconcile manages namespace watches for the modified KubernetesDiscovery object.
func (w *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	key := watcherID(request.NamespacedName)
	existing, hasExisting := w.watchers[key]

	var kd v1alpha1.KubernetesDiscovery
	err := w.ctrlClient.Get(ctx, request.NamespacedName, &kd)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !kd.ObjectMeta.DeletionTimestamp.IsZero() {
		// spec was deleted - just clean up any watches and we're done
		if hasExisting {
			w.teardown(key)
			w.cleanupAbandonedNamespaces()
		}
		return ctrl.Result{}, nil
	}

	if !hasExisting || !equality.Semantic.DeepEqual(existing.spec, kd.Spec) {
		if err := w.addOrReplace(ctx, key, &kd); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile %s:%s: %v",
				key.Namespace, key.Name, err)
		}
	}

	return ctrl.Result{}, nil
}

func (w *Reconciler) addOrReplace(ctx context.Context, key watcherID, kd *store.KubernetesDiscovery) error {
	var extraSelectors []labels.Selector
	for _, s := range kd.Spec.ExtraSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&s)
		if err != nil {
			return fmt.Errorf("invalid label selectors: %v", err)
		}
		extraSelectors = append(extraSelectors, selector)
	}

	if _, ok := w.watchers[key]; ok {
		// if a watcher already exists, just tear it down and we'll set it up from scratch so that
		// we don't have to diff a bunch of different pieces
		w.teardown(key)
	}

	currentNamespaces, currentUIDs := namespacesAndUIDsFromSpec(kd.Spec.Watches)
	for namespace := range currentNamespaces {
		w.setupNamespaceWatch(ctx, namespace, key)
	}

	for watchUID := range currentUIDs {
		w.setupUIDWatch(ctx, watchUID, key)
	}

	pw := watcher{
		spec:           *kd.Spec.DeepCopy(),
		startTime:      time.Now(),
		extraSelectors: extraSelectors,
	}

	w.watchers[key] = pw

	// now that we've finished setup, ensure that any namespaces of which this was the last watcher
	// have their watch stopped
	w.cleanupAbandonedNamespaces()

	// always emit an update status event so that any Pods that the reconciler _already_ knows about get populated
	// this is extremely common as usually the reconciler receives the Pod event before the caller is able to
	// propagate their watch via the KubernetesDiscovery spec to be seen here
	if err := w.updateStatus(ctx, w.store, key); err != nil {
		return fmt.Errorf("failed to update KubernetesDiscovery %q: %v", key, err)
	}
	return nil
}

// teardown removes the watcher from all namespace + UIDs it was watching.
//
// By design, teardown does NOT clean up any watches for namespaces that no longer have any active watchers.
// This is done by calling cleanupAbandonedNamespaces explicitly, which allows addOrReplace to have simpler logic
// by always calling teardown on a resource, then treating it as "new" and only cleaning up after it has (re-)added
// the watches without needlessly removing + recreating the lower-level namespace watch.
func (w *Reconciler) teardown(key watcherID) {
	watcher := w.watchers[key]
	namespaces, uids := namespacesAndUIDsFromSpec(watcher.spec.Watches)
	for namespace := range namespaces {
		delete(w.watchedNamespaces[namespace].watchers, key)
	}

	for uid := range uids {
		delete(w.uidWatchers[uid], key)
	}

	delete(w.watchers, key)
}

// cleanupAbandonedNamespaces removes the watch on any namespaces that no longer have any active watchers.
//
// mu must be held by caller.
//
// See watchedNamespaces for more details (for efficiency, we don't want duplicative namespace watches).
func (w *Reconciler) cleanupAbandonedNamespaces() {
	for ns, watcher := range w.watchedNamespaces {
		if len(watcher.watchers) == 0 {
			watcher.cancel()
			delete(w.watchedNamespaces, ns)
		}
	}
}

// setupNamespaceWatch creates a namespace watch if necessary and adds a key to the list of watchers for it.
//
// mu must be held by caller.
//
// It is idempotent:
// 	* If no watch for the namespace exists, it is created and the given key is the sole watcher
// 	* If a watch for the namespace exists but the given key is not in the watcher list, it is added
// 	* If a watch for the namespace exists and the given key is already in the watcher list, it no-ops
//
// This ensures it can be safely called by reconcile on each invocation for any namespace that the watcher cares about.
// Additionally, for efficiency, duplicative watches on the same namespace will not be created; see watchedNamespaces
// for more details.
func (w *Reconciler) setupNamespaceWatch(ctx context.Context, ns k8s.Namespace, watcherKey watcherID) {
	if watcher, ok := w.watchedNamespaces[ns]; ok {
		// already watching this namespace -- just add this watcher to the list for cleanup tracking
		watcher.watchers[watcherKey] = true
		return
	}

	ch, err := w.kCli.WatchPods(ctx, ns)
	if err != nil {
		err = errors.Wrapf(err, "Error watching pods. Are you connected to kubernetes?\nTry running `kubectl get pods -n %q`", ns)
		w.store.Dispatch(store.NewErrorAction(err))
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	w.watchedNamespaces[ns] = nsWatch{
		watchers: map[watcherID]bool{watcherKey: true},
		cancel:   cancel,
	}

	go w.dispatchPodChangesLoop(ctx, ch, w.store)
}

// setupUIDWatch registers a watcher to receive updates for any Pods transitively owned by this UID (or that exactly
// match this UID).
//
// mu must be held by caller.
func (w *Reconciler) setupUIDWatch(_ context.Context, uid types.UID, watcherID watcherID) {
	if w.uidWatchers[uid][watcherID] {
		return
	}

	// add this key as a watcher for the UID
	uidWatchers, ok := w.uidWatchers[uid]
	if !ok {
		uidWatchers = make(watcherSet)
		w.uidWatchers[uid] = uidWatchers
	}
	uidWatchers[watcherID] = true
}

// updateStatus builds the latest status for the given KubernetesDiscovery spec key and persists it.
//
// mu must be held by caller.
//
// If the status has not changed since the last status update performed (by the Reconciler), it will be skipped.
// Additionally, if the spec that is being used by the watcher does not match the current spec from the server, it
// will be skipped to avoid stale/inconsistent status data.
func (w *Reconciler) updateStatus(ctx context.Context, st Dispatcher, watcherID watcherID) error {
	watcher := w.watchers[watcherID]
	status := w.buildStatus(ctx, watcher)
	if equality.Semantic.DeepEqual(watcher.lastUpdate, status) {
		// the status hasn't changed - avoid a spurious update
		return nil
	}

	key := types.NamespacedName(watcherID)
	var kd v1alpha1.KubernetesDiscovery
	if err := w.ctrlClient.Get(ctx, key, &kd); err != nil {
		if apierrors.IsNotFound(err) {
			// if the spec got deleted, there's nothing to update
			return nil
		}
		return fmt.Errorf("failed to get KubernetesDiscovery status for %q: %v", watcherID, err)
	}

	if !equality.Semantic.DeepEqual(watcher.spec, kd.Spec) {
		// the spec has changed and we haven't reconciled that change yet; this state update is
		// outdated, so just discard it
		return nil
	}

	kd.Status = status
	if err := w.ctrlClient.Status().Update(ctx, &kd); err != nil {
		if apierrors.IsNotFound(err) {
			// similar to above but for the event that it gets deleted between get + update
			return nil
		}
		return fmt.Errorf("failed to update KubernetesDiscovery status for %q: %v", watcherID, err)
	}

	st.Dispatch(k8swatch.NewKubernetesDiscoveryUpdateStatusAction(&kd))
	w.restartDetector.Detect(st, watcher.lastUpdate, kd)
	watcher.lastUpdate = *status.DeepCopy()
	w.watchers[watcherID] = watcher
	return nil
}

// buildStatus creates the current state for the given KubernetesDiscovery object key.
//
// mu must be held by caller.
func (w *Reconciler) buildStatus(ctx context.Context, watcher watcher) v1alpha1.KubernetesDiscoveryStatus {
	seenPodUIDs := k8s.NewUIDSet()
	var pods []v1alpha1.Pod
	maybeTrackPod := func(pod *v1.Pod, ancestorUID types.UID) {
		if pod == nil || seenPodUIDs.Contains(pod.UID) {
			return
		}
		seenPodUIDs.Add(pod.UID)
		pods = append(pods, *k8sconv.Pod(ctx, pod, ancestorUID))
	}

	for i := range watcher.spec.Watches {
		watchUID := types.UID(watcher.spec.Watches[i].UID)
		if watchUID == "" || seenPodUIDs.Contains(watchUID) {
			continue
		}
		// UID could either refer directly to a Pod OR its ancestor (e.g. Deployment)
		maybeTrackPod(w.knownPods[watchUID], watchUID)
		for podUID := range w.knownDescendentPodUIDs[watchUID] {
			maybeTrackPod(w.knownPods[podUID], watchUID)
		}
	}

	// TODO(milas): we should only match against Pods in namespaces referenced by the WatchRefs for this spec
	if len(watcher.spec.ExtraSelectors) != 0 {
		for podUID, pod := range w.knownPods {
			if seenPodUIDs.Contains(podUID) {
				// because we're brute forcing this - make an attempt to
				// reduce work and not bother to try matching on Pods that
				// have already been seen
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

	return v1alpha1.KubernetesDiscoveryStatus{
		MonitorStartTime: apis.NewMicroTime(watcher.startTime),
		Pods:             pods,
	}
}

func (w *Reconciler) upsertPod(pod *v1.Pod) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.knownPods[pod.UID] = pod
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
func (w *Reconciler) triagePodTree(pod *v1.Pod, objTree k8s.ObjectRefTree) []triageResult {
	uid := pod.UID

	// Set up the descendent pod UID index
	for _, ownerUID := range objTree.UIDs() {
		if uid == ownerUID {
			continue
		}

		set, ok := w.knownDescendentPodUIDs[ownerUID]
		if !ok {
			set = k8s.NewUIDSet()
			w.knownDescendentPodUIDs[ownerUID] = set
		}
		set.Add(uid)
	}

	seenWatchers := make(map[watcherID]bool)
	var results []triageResult

	// Find any watchers that have a ref to a UID in the object tree (i.e. the Pod itself or a transitive owner)
	for _, ownerUID := range objTree.UIDs() {
		for watcherID := range w.uidWatchers[ownerUID] {
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

func (w *Reconciler) handlePodChange(ctx context.Context, pod *v1.Pod, st Dispatcher) {
	objTree, err := w.ownerFetcher.OwnerTreeOf(ctx, k8s.NewK8sEntity(pod))
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	triageResults := w.triagePodTree(pod, objTree)

	for i := range triageResults {
		watcherID := triageResults[i].watcherID
		if err := w.updateStatus(ctx, st, watcherID); err != nil {
			st.Dispatch(store.NewErrorAction(err))
			return
		}
	}
}

func (w *Reconciler) handlePodDelete(ctx context.Context, st Dispatcher, namespace k8s.Namespace, name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var podUID types.UID
	for uid, pod := range w.knownPods {
		if pod.Namespace == namespace.String() && pod.Name == name {
			delete(w.knownPods, uid)
			podUID = uid
			break
		}
	}

	if podUID == "" {
		// this pod wasn't known/tracked
		return
	}

	// because we don't know if any watchers matched on this Pod by label previously,
	// trigger an update on every watcher, which will return early if it didn't change
	for watcherID := range w.watchers {
		if err := w.updateStatus(ctx, st, watcherID); err != nil {
			st.Dispatch(store.NewErrorAction(err))
			return
		}
	}
}

func (w *Reconciler) dispatchPodChangesLoop(ctx context.Context, ch <-chan k8s.ObjectUpdate, st Dispatcher) {
	for {
		select {
		case obj, ok := <-ch:
			if !ok {
				return
			}

			pod, ok := obj.AsPod()
			if ok {
				w.upsertPod(pod)

				go w.handlePodChange(ctx, pod, w.store)
				continue
			}

			namespace, name, ok := obj.AsDeletedKey()
			if ok {
				go w.handlePodDelete(ctx, st, namespace, name)
				continue
			}
		case <-ctx.Done():
			return
		}
	}
}

type namespaceSet map[k8s.Namespace]bool

func namespacesAndUIDsFromSpec(watches []v1alpha1.KubernetesWatchRef) (namespaceSet, k8s.UIDSet) {
	seenNamespaces := make(namespaceSet)
	seenUIDs := k8s.NewUIDSet()

	for i := range watches {
		seenNamespaces[k8s.Namespace(watches[i].Namespace)] = true
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
