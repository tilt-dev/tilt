package k8swatch

import (
	"context"
	"sort"
	"sync"

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

	// extraSelectors is a mapping of KubernetesDiscovery keys to extra label selectors for matching
	// pods that don't transitively matched a known UID.
	extraSelectors map[watcherID][]labels.Selector

	// watchedNamespaces tracks the namespaces that are being observed for Pod events.
	//
	// For efficiency, a single watch is created for a given namespace and keys of watchers
	// are tracked; once there are no more watchers, cleanupAbandonedNamespaces will cancel
	// the watch.
	watchedNamespaces map[k8s.Namespace]nsWatch

	// uidWatchers are the KubernetesDiscovery objects that have a watch ref for a particular K8s UID,
	// and so will receive events for changes to it.
	uidWatchers map[types.UID]watcherSet

	// kubernetesDiscoveryManifest is a temporary mapping of KubernetesDiscovery keys to associated manifests.
	//
	// Currently, PodChangeAction is associated with a particular manifest, so this allows for
	// dispatching events without changing that logic. This will be eliminated once this dispatches
	// KubernetesDiscoveryUpdateStatusAction instead and the reducers will be responsible for associating
	// it back to a manifest (or ignoring it if none).
	kubernetesDiscoveryManifest map[watcherID]model.ManifestName

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
		kCli:                        kCli,
		ownerFetcher:                ownerFetcher,
		extraSelectors:              make(map[watcherID][]labels.Selector),
		watchedNamespaces:           make(map[k8s.Namespace]nsWatch),
		uidWatchers:                 make(map[types.UID]watcherSet),
		kubernetesDiscoveryManifest: make(map[watcherID]model.ManifestName),
		knownDescendentPodUIDs:      make(map[types.UID]k8s.UIDSet),
		knownPods:                   make(map[types.UID]*v1.Pod),
	}
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
	for _, s := range w.extraSelectors[watcherID(key)] {
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
		delete(w.kubernetesDiscoveryManifest, key)
		delete(w.extraSelectors, key)
		for _, watcher := range w.watchedNamespaces {
			// remove this key from the list of watchers; if it was the last watcher, the
			// namespace watch will be cleaned up afterwards
			delete(watcher.watchers, key)
		}
		for _, watchers := range w.uidWatchers {
			delete(watchers, key)
		}
		return
	}

	if manifestName, ok := kd.Annotations[v1alpha1.AnnotationManifest]; ok && manifestName != "" {
		// PodChangeAction is currently tied to manifest so keep a mapping of API objects to manifests
		// (this will be removed shortly once PodChangeAction -> KubernetesDiscoveryUpdateStatusAction)
		w.kubernetesDiscoveryManifest[key] = model.ManifestName(manifestName)
	}

	w.extraSelectors[key] = nil
	for _, labelValues := range kd.Spec.ExtraSelectors {
		ls := selectorFromLabels(labelValues)
		if !ls.Empty() {
			w.extraSelectors[key] = append(w.extraSelectors[key], ls)
		}
	}

	seenUIDs := make(map[types.UID]bool)
	seenNamespaces := make(map[k8s.Namespace]bool)
	for _, toWatch := range kd.Spec.Watches {
		// ensure a namespace watch exists + this watcher is referenced by it
		namespace := k8s.Namespace(toWatch.Namespace)
		seenNamespaces[namespace] = true
		w.setupNamespaceWatch(ctx, st, namespace, key)

		// a watch ref might not have a UID (either resources haven't been deployed yet OR spec relies on labels)
		if toWatch.UID != "" {
			watchUID := types.UID(toWatch.UID)
			seenUIDs[watchUID] = true
			w.setupUIDWatch(ctx, st, watchUID, key)
		}
	}

	for ns, nsWatch := range w.watchedNamespaces {
		if !seenNamespaces[ns] {
			// remove this key from the list of watchers; if it was the last watcher, the
			// namespace watch will be cleaned up afterwards
			delete(nsWatch.watchers, key)
		}
	}

	for uid, watchers := range w.uidWatchers {
		if !seenUIDs[uid] {
			delete(watchers, key)
		}
	}
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

	mn, ok := w.kubernetesDiscoveryManifest[watcherID]
	if !ok {
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
	for key, selectors := range w.extraSelectors {
		for _, selector := range selectors {
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

		mn := w.kubernetesDiscoveryManifest[key]
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

func selectorFromLabels(labelValues []v1alpha1.LabelValue) labels.Selector {
	ls := make(labels.Set, len(labelValues))
	for _, l := range labelValues {
		ls[l.Label] = l.Value
	}
	return ls.AsSelector()
}
