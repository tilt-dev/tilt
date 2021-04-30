package k8swatch

import (
	"context"
	"fmt"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/k8s"
)

// watcherID is to disambiguate between K8s object keys and tilt-apiserver KubernetesDiscovery object keys.
type watcherID types.NamespacedName
type watcherSet map[watcherID]bool

type PodWatcher struct {
	kCli         k8s.Client
	ownerFetcher k8s.OwnerFetcher
	// mu should be held throughout OnChange; helper methods used by it expect it to be held.
	// Any helper methods for the dispatch loop should claim the lock as needed.
	mu sync.Mutex

	// watchedNamespaces tracks the namespaces that are being observed for Pod events.
	//
	// For efficiency, a single watch is created for a given namespace and keys of watchers
	// are tracked; once there are no more watchers, cleanupAbandonedNamespaces will cancel
	// the watch.
	watchedNamespaces map[k8s.Namespace]nsWatch

	// watchers reflects the current state of the PodWatcher namespace + UID watches.
	//
	// On reconcile, if the latest spec differs from what's tracked here, it will be acted upon.
	watchers map[watcherID]watcher

	// uidWatchers are the KubernetesDiscovery objects that have a watch ref for a particular K8s UID,
	// and so will receive events for changes to it.
	uidWatchers map[types.UID]watcherSet

	// An index that maps the UID of Kubernetes resources to the UIDs of
	// all pods that they own (transitively).
	//
	// For example, a Deployment UID might contain a set of N pod UIDs.
	knownDescendentPodUIDs map[types.UID]k8s.UIDSet

	// An index of all the known pods, by UID
	knownPods map[types.UID]*v1.Pod
}

func NewPodWatcher(kCli k8s.Client, ownerFetcher k8s.OwnerFetcher) *PodWatcher {
	return &PodWatcher{
		kCli:                   kCli,
		ownerFetcher:           ownerFetcher,
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
	spec *v1alpha1.KubernetesDiscoverySpec
	// manifestName is a temporary mapping of a KubernetesDiscovery key to an associated manifest (if any).
	//
	// Currently, PodChangeAction is associated with a particular manifest, so this allows for
	// dispatching events without changing that logic. This will be eliminated once this dispatches
	// KubernetesDiscoveryUpdateStatusAction instead and the reducers will be responsible for associating
	// it back to a manifest (or ignoring it if none).
	manifestName model.ManifestName
	// extraSelectors are label selectors used to match pods that don't transitively match any known UID.
	extraSelectors []labels.Selector
}

// nsWatch tracks the watchers for the given namespace and allows the watch to be canceled.
type nsWatch struct {
	watchers map[watcherID]bool
	cancel   context.CancelFunc
}

// OnChange reconciles based on changes to KubernetesDiscovery objects.
func (w *PodWatcher) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if len(summary.KubernetesDiscoveries.Changes) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for key := range summary.KubernetesDiscoveries.Changes {
		w.reconcile(ctx, st, watcherID(key))
	}

	w.cleanupAbandonedNamespaces()
}

// HasNamespaceWatch returns true if the key is a watcher for the given namespace.
//
// This is intended for use in tests.
func (w *PodWatcher) HasNamespaceWatch(key types.NamespacedName, ns k8s.Namespace) bool {
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
func (w *PodWatcher) HasUIDWatch(key types.NamespacedName, uid types.UID) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.uidWatchers[uid][watcherID(key)]
}

// ExtraSelectors returns the extra selectors for a given KubernetesDiscovery object.
//
// This is intended for use in tests.
func (w *PodWatcher) ExtraSelectors(key types.NamespacedName) []labels.Selector {
	w.mu.Lock()
	defer w.mu.Unlock()
	var ret []labels.Selector
	for _, s := range w.watchers[watcherID(key)].extraSelectors {
		ret = append(ret, s.DeepCopySelector())
	}
	return ret
}

// reconcile manages namespace watches for the modified KubernetesDiscovery object.
func (w *PodWatcher) reconcile(ctx context.Context, st store.RStore, key watcherID) {
	state := st.RLockState()
	kd := state.KubernetesDiscoveries[types.NamespacedName(key)]
	st.RUnlockState()

	if kd == nil {
		// this object doesn't exist in store (i.e. it was deleted) so clean up
		// any watches + other metadata
		w.teardown(key)
		return
	}

	if existing := w.watchers[key]; existing.spec == nil || !equality.Semantic.DeepEqual(existing.spec, kd.Spec) {
		if err := w.addOrReplace(ctx, st, key, kd); err != nil {
			// currently all errors are fatal; once this writes to API server it will become more lenient
			st.Dispatch(store.NewErrorAction(fmt.Errorf("failed to reconcile %s:%s: %v",
				key.Namespace, key.Name, err)))
		}
	}
}

func (w *PodWatcher) addOrReplace(ctx context.Context, st store.RStore, key watcherID, kd *store.KubernetesDiscovery) error {
	var extraSelectors []labels.Selector
	for _, s := range kd.Spec.ExtraSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&s)
		if err != nil {
			return fmt.Errorf("invalid label selectors: %v", err)
		}
		extraSelectors = append(extraSelectors, selector)
	}

	currentNamespaces, currentUIDs := namespacesAndUIDsFromSpec(kd.Spec.Watches)
	// handle cleanup from the previous version (if any) - this is very similar to teardown() except that it only
	// removes stale entries vs all; currently, it's not feasible to teardown() followed by setup() because during
	// UID setup, events are re-broadcast ONLY if it's being newly tracked. this restriction will go away once this
	// logic switches to dispatching KubernetesDiscoveryUpdateStatusAction as it will _always_ dispatch the event,
	// so can simplify logic by just cleaning up the old before setting up the new.
	if prev, ok := w.watchers[key]; ok {
		prevNamespaces, prevUIDs := namespacesAndUIDsFromSpec(prev.spec.Watches)
		for namespace := range prevNamespaces {
			if !currentNamespaces[namespace] {
				// if this is the last watcher on the namespace, cleanupAbandonedNamespaces will handle actually
				// canceling the watch later
				delete(w.watchedNamespaces[namespace].watchers, key)
			}
		}

		for uid := range prevUIDs {
			if !currentUIDs.Contains(uid) {
				delete(w.uidWatchers[uid], key)
			}
		}
	}

	for namespace := range currentNamespaces {
		w.setupNamespaceWatch(ctx, st, namespace, key)
	}

	for watchUID := range currentUIDs {
		w.setupUIDWatch(ctx, st, watchUID, key)
	}

	pw := watcher{
		spec:           kd.Spec.DeepCopy(),
		manifestName:   model.ManifestName(kd.Annotations[v1alpha1.AnnotationManifest]),
		extraSelectors: extraSelectors,
	}

	w.watchers[key] = pw
	return nil
}

// teardown removes the watcher from all namespace + UIDs it was watching.
func (w *PodWatcher) teardown(key watcherID) {
	watcher := w.watchers[key]
	if watcher.spec == nil {
		return
	}

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
func (w *PodWatcher) cleanupAbandonedNamespaces() {
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
func (w *PodWatcher) setupNamespaceWatch(ctx context.Context, st store.RStore, ns k8s.Namespace, watcherKey watcherID) {
	if watcher, ok := w.watchedNamespaces[ns]; ok {
		// already watching this namespace -- just add this watcher to the list for cleanup tracking
		watcher.watchers[watcherKey] = true
		return
	}

	ch, err := w.kCli.WatchPods(ctx, ns)
	if err != nil {
		err = errors.Wrapf(err, "Error watching pods. Are you connected to kubernetes?\nTry running `kubectl get pods -n %q`", ns)
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	w.watchedNamespaces[ns] = nsWatch{
		watchers: map[watcherID]bool{watcherKey: true},
		cancel:   cancel,
	}

	go w.dispatchPodChangesLoop(ctx, ch, st)
}

// setupUIDWatch handles dispatching events to a watcher immediately for a UID it has not seen before.
//
// mu must be held by caller.
//
// Very frequently, Pod events (as dispatched by K8s) for a UID have already been seen by the PodWatcher _before_ the
// consumer (e.g. the ManifestSubscriber which generates KubernetesDiscovery specs) was able to propagate the UID for
// discovery, so a Pod change event is sent to allow it to immediately populate an already known (to PodWatcher) Pod.
func (w *PodWatcher) setupUIDWatch(ctx context.Context, st store.RStore, uid types.UID, watcherID watcherID) {
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

	mn := w.watchers[watcherID].manifestName
	if mn == "" {
		// currently actions are tied to manifest, so there's nothing more to do if this spec
		// didn't originate from a manifest
		return
	}

	pod, ok := w.knownPods[uid]
	if ok {
		st.Dispatch(NewPodChangeAction(k8sconv.Pod(ctx, pod), mn, uid))
		// since this UID matched a known pod, there's no reason to look at descendants
		return
	}

	descendants := w.knownDescendentPodUIDs[uid]
	for podUID := range descendants {
		pod, ok := w.knownPods[podUID]
		if ok {
			st.Dispatch(NewPodChangeAction(k8sconv.Pod(ctx, pod), mn, uid))
		}
	}
}

func (w *PodWatcher) upsertPod(pod *v1.Pod) {
	w.mu.Lock()
	defer w.mu.Unlock()

	uid := pod.UID
	w.knownPods[uid] = pod
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
// If it does not transitively belong to any watched UIDs, it will be compared against any
// extra selectors from the specs instead. Only the _first_ match receives an event; this is
// to ensure that more than one manifest doesn't see it.
//
// Even if the Pod doesn't match any KubernetesDiscovery spec, it's still kept in local state,
// so we can match it later if a KubernetesDiscovery spec is modified to match it; this is actually
// extremely common because new Pods are typically observed here _before_ the respective
// KubernetesDiscovery spec update propagates (see setupUIDWatch).
func (w *PodWatcher) triagePodTree(pod *v1.Pod, objTree k8s.ObjectRefTree) []triageResult {
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

	var results []triageResult

	// Find any watchers that have a ref to a UID in the object tree (i.e. the Pod itself or a transitive owner)
	for _, ownerUID := range objTree.UIDs() {
		for watcherID := range w.uidWatchers[ownerUID] {
			results = append(results, triageResult{watcherID: watcherID, ancestorUID: ownerUID})
		}
	}

	if len(results) != 0 {
		// TODO(milas): it doesn't totally make sense that extra selectors only apply if no other watcher matched them,
		// 	but if this constraint is removed, it'll open the door for >1 manifest to see the same Pod, which causes
		// 	problems in other parts of the engine
		return results
	}

	// If we can't find the key based on owner, check to see if the pod any
	// of the extra selectors.
	//
	// NOTE(nick): This code might be totally obsolete now that we triage
	// pods by owner UID. It's meant to handle CRDs, but most CRDs should
	// set owner reference appropriately.
	podLabels := labels.Set(pod.ObjectMeta.GetLabels())
	for key, watcher := range w.watchers {
		for _, selector := range watcher.extraSelectors {
			if selector.Matches(podLabels) {
				// there is no ancestorUID since this was a label match
				results = append(results, triageResult{watcherID: key, ancestorUID: ""})
			}
		}
	}

	if len(results) > 1 {
		// TODO(milas): similar to the reason for the early return, in the event that we fell back to label selectors,
		// 	multiple manifests might match, which can cause trouble in the engine downstream, so arbitrarily (but
		// 	deterministically) pick ONE to get the event for now
		sort.SliceStable(results, func(i, j int) bool {
			return results[i].watcherID.Name < results[j].watcherID.Name
		})
		return results[:1]
	}

	return results
}

func (w *PodWatcher) dispatchPodChange(ctx context.Context, pod *v1.Pod, st store.RStore) {
	objTree, err := w.ownerFetcher.OwnerTreeOf(ctx, k8s.NewK8sEntity(pod))
	if err != nil {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	triageResults := w.triagePodTree(pod, objTree)

	for i := range triageResults {
		key := triageResults[i].watcherID
		ancestorUID := triageResults[i].ancestorUID

		mn := w.watchers[key].manifestName
		if mn == "" {
			// currently, PodChangeAction is tied to a manifest, so only Pods that are associated
			// with a KubernetesDiscovery spec that originated from a manifest can have events dispatched
			// (this will change imminently once PodChangeAction -> KubernetesDiscoveryUpdateStatusAction)
			return
		}

		freshPod, ok := w.knownPods[pod.UID]
		if ok {
			st.Dispatch(NewPodChangeAction(k8sconv.Pod(ctx, freshPod), mn, ancestorUID))
		}
	}
}

func (w *PodWatcher) dispatchPodChangesLoop(ctx context.Context, ch <-chan k8s.ObjectUpdate, st store.RStore) {
	for {
		select {
		case obj, ok := <-ch:
			if !ok {
				return
			}

			pod, ok := obj.AsPod()
			if ok {
				w.upsertPod(pod)

				go w.dispatchPodChange(ctx, pod, st)
				continue
			}

			namespace, name, ok := obj.AsDeletedKey()
			if ok {
				go st.Dispatch(NewPodDeleteAction(k8s.PodID(name), namespace))
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
