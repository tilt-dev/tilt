package k8swatch

import (
	"context"
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

type PodWatcher struct {
	kCli         k8s.Client
	ownerFetcher k8s.OwnerFetcher
	// mu should be held throughout OnChange; helper methods used by it expect it to be held.
	// Any helper methods for the dispatch loop should claim the lock as needed.
	mu sync.Mutex

	// extraSelectors is a mapping of KubernetesDiscovery keys to extra label selectors for matching
	// pods that don't transitively matched a known UID.
	extraSelectors map[types.NamespacedName][]labels.Selector

	// watchedNamespaces tracks the namespaces that are being observed for Pod events.
	//
	// For efficiency, a single watch is created for a given namespace and keys of watchers
	// are tracked; once there are no more watchers, cleanupAbandonedNamespaces will cancel
	// the watch.
	watchedNamespaces map[k8s.Namespace]nsWatch

	// claimedUIDs enforces a unique association for a particular UID to a watcher.
	//
	// This is a temporary restriction to align with behavior that meant a given UID could
	// be referenced by multiple manifests but, to avoid duplicate events, only associate
	// events with one of them. Once PodChangeAction -> KubernetesDiscoveryUpdateStatusAction, this
	// restriction will be lifted and ManifestSubscriber will instead handle "claims" for
	// manifests, but the general restriction will no longer apply at an API level.
	claimedUIDs map[types.UID]types.NamespacedName

	// kubernetesDiscoveryManifest is a temporary mapping of KubernetesDiscovery keys to associated manifests.
	//
	// Currently, PodChangeAction is associated with a particular manifest, so this allows for
	// dispatching events without changing that logic. This will be eliminated once this dispatches
	// KubernetesDiscoveryUpdateStatusAction instead and the reducers will be responsible for associating
	// it back to a manifest (or ignoring it if none).
	kubernetesDiscoveryManifest map[types.NamespacedName]model.ManifestName

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
		extraSelectors:              make(map[types.NamespacedName][]labels.Selector),
		watchedNamespaces:           make(map[k8s.Namespace]nsWatch),
		claimedUIDs:                 make(map[types.UID]types.NamespacedName),
		kubernetesDiscoveryManifest: make(map[types.NamespacedName]model.ManifestName),
		knownDescendentPodUIDs:      make(map[types.UID]k8s.UIDSet),
		knownPods:                   make(map[types.UID]*v1.Pod),
	}
}

// nsWatch tracks the watchers for the given namespace and allows the watch to be canceled.
type nsWatch struct {
	watchers map[types.NamespacedName]bool
	cancel   context.CancelFunc
}

func selectorFromLabels(labelValues []v1alpha1.LabelValue) labels.Selector {
	ls := make(labels.Set, len(labelValues))
	for _, l := range labelValues {
		ls[l.Label] = l.Value
	}
	return ls.AsSelector()
}

// OnChange reconciles based on changes to KubernetesDiscovery objects.
func (w *PodWatcher) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if len(summary.KubernetesDiscoveries.Changes) == 0 {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	for key := range summary.KubernetesDiscoveries.Changes {
		w.reconcile(ctx, st, key)
	}

	w.cleanupAbandonedNamespaces()
}

// HasNamespaceWatch returns true if the key is a watcher for the given namespace.
func (w *PodWatcher) HasNamespaceWatch(key types.NamespacedName, ns k8s.Namespace) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	nsWatch, ok := w.watchedNamespaces[ns]
	if !ok {
		return false
	}
	return nsWatch.watchers[key]
}

// ExtraSelectors returns the extra selectors for a given KubernetesDiscovery object.
//
// This is intended for use in tests.
func (w *PodWatcher) ExtraSelectors(key types.NamespacedName) []labels.Selector {
	w.mu.Lock()
	defer w.mu.Unlock()
	var ret []labels.Selector
	for _, s := range w.extraSelectors[key] {
		ret = append(ret, s.DeepCopySelector())
	}
	return ret
}

// Claimant returns the key that has uniquely claimed a particular K8s entity UID to prevent
// duplicate events.
//
// This is intended for use in tests.
func (w *PodWatcher) Claimant(uid types.UID) types.NamespacedName {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.claimedUIDs[uid]
}

// reconcile manages namespace watches for the modified KubernetesDiscovery object.
func (w *PodWatcher) reconcile(ctx context.Context, st store.RStore, key types.NamespacedName) {
	state := st.RLockState()
	kd := state.KubernetesDiscoveries[key]
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
		for uid, k := range w.claimedUIDs {
			if k == key {
				delete(w.claimedUIDs, uid)
			}
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
		namespace := k8s.Namespace(toWatch.Namespace)
		if !seenNamespaces[namespace] {
			w.setupWatch(ctx, st, namespace, key)
			seenNamespaces[namespace] = true
		}

		id := types.UID(toWatch.UID)
		if id == "" || seenUIDs[id] {
			continue
		}
		seenUIDs[id] = true

		if claimant, ok := w.claimedUIDs[id]; !ok || claimant != key {
			// either this UID has not been claimed (so it's new to Tilt)
			// or the claim has changed (so it's new to this KubernetesDiscovery object)
			w.setupNewUID(ctx, st, id, key)
		}
	}

	for ns, nsWatch := range w.watchedNamespaces {
		if !seenNamespaces[ns] {
			// remove this key from the list of watchers; if it was the last watcher, the
			// namespace watch will be cleaned up afterwards
			delete(nsWatch.watchers, key)
		}
	}

	for uid, claimant := range w.claimedUIDs {
		if claimant == key && !seenUIDs[uid] {
			// remove any claims on UIDs that vanished; most likely the object was destroyed,
			// but it's also possible for two manifests to reference the same UID, so this
			// ensures that if another manifest still references the UID, it will be free to
			// take the claim the next time it's reconciled (this isn't perfect, but it's
			// a pathological case that will be better resolved soon)
			delete(w.claimedUIDs, uid)
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

// setupWatch creates a namespace watch if necessary and adds a key to the list of watchers for it.
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
func (w *PodWatcher) setupWatch(ctx context.Context, st store.RStore, ns k8s.Namespace, watcherKey types.NamespacedName) {
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
		watchers: map[types.NamespacedName]bool{watcherKey: true},
		cancel:   cancel,
	}

	go w.dispatchPodChangesLoop(ctx, ch, st)
}

// setupNewUID handles dispatching events immediately for a new UID.
//
// mu must be held by caller.
//
// There are two cases of a new UID: one common and one esoteric. The former is what is typically thought of
// as a new UID: it has just been added for discovery to a KubernetesDiscovery spec. Very frequently, Pod events for
// this UID have already been seen by the watcher here (as dispatched by K8s) _before_ the consumer was able to
// propagate the UID for discovery, so a Pod change event is sent to allow it to populate.
//
// The latter case is when a UID changes "claims" between two manifests. This is a strange edge case, but the
// same reasoning applies: the new consumer still needs to get data populated, so an event is (re-)dispatched
// but targeted at the new claimant.
func (w *PodWatcher) setupNewUID(ctx context.Context, st store.RStore, uid types.UID, claimedByKey types.NamespacedName) {
	w.claimedUIDs[uid] = claimedByKey
	mn, ok := w.kubernetesDiscoveryManifest[claimedByKey]
	if !ok {
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

// triagePodTree checks to see if this Pod corresponds to any of the KubernetesDiscovery objects.
//
// Currently, we do this by comparing the Pod UID and its owner UIDs against watched UIDs from KubernetesDiscovery specs.
// If it does not transitively belong to watched UID, it will be compared against any extra selectors from the
// specs instead.
//
// If the Pod doesn't match any KubernetesDiscovery spec, it's kept in local  state, so we can match it later if a
// KubernetesDiscovery spec is modified to match it; this is actually extremely common because new Pods are typically
// observed here _before_ the respective KubernetesDiscovery spec update propagates (see setupNewUID).
func (w *PodWatcher) triagePodTree(pod *v1.Pod, objTree k8s.ObjectRefTree) (types.NamespacedName, types.UID) {
	w.mu.Lock()
	defer w.mu.Unlock()

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

	// Find the manifest name
	for _, ownerUID := range objTree.UIDs() {
		key, ok := w.claimedUIDs[ownerUID]
		if ok {
			return key, ownerUID
		}
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
				return key, ""
			}
		}
	}
	return types.NamespacedName{}, ""
}

func (w *PodWatcher) dispatchPodChange(ctx context.Context, pod *v1.Pod, st store.RStore) {
	objTree, err := w.ownerFetcher.OwnerTreeOf(ctx, k8s.NewK8sEntity(pod))
	if err != nil {
		return
	}

	key, ancestorUID := w.triagePodTree(pod, objTree)

	w.mu.Lock()
	defer w.mu.Unlock()
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
