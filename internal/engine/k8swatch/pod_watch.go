package k8swatch

import (
	"context"
	"fmt"
	"sync"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/k8s"
)

type PodWatcher struct {
	kCli         k8s.Client
	ownerFetcher k8s.OwnerFetcher

	mu                sync.RWMutex
	watches           []PodWatch
	knownDeployedUIDs map[types.UID]model.ManifestName

	// An index that maps the UID of Kubernetes resources to the UIDs of
	// all pods that they own (transitively).
	//
	// For example, a Deployment UID might contain a set of N pod UIDs.
	knownDescendentPodUIDs map[types.UID]store.UIDSet

	// An index of all the known pods, by UID
	knownPods map[types.UID]*v1.Pod
}

func NewPodWatcher(kCli k8s.Client, ownerFetcher k8s.OwnerFetcher) *PodWatcher {
	return &PodWatcher{
		kCli:                   kCli,
		ownerFetcher:           ownerFetcher,
		knownDeployedUIDs:      make(map[types.UID]model.ManifestName),
		knownDescendentPodUIDs: make(map[types.UID]store.UIDSet),
		knownPods:              make(map[types.UID]*v1.Pod),
	}
}

type PodWatch struct {
	name   model.ManifestName
	labels labels.Selector
	cancel context.CancelFunc
}

func (pw PodWatch) Equal(other PodWatch) bool {
	return pw.name == other.name && k8s.SelectorEqual(pw.labels, other.labels)
}

// returns all elements of `a` that are not in `b`
func subtract(a, b []PodWatch) []PodWatch {
	var ret []PodWatch
	// silly O(n^3) diff here on assumption that lists will be trivially small
	for _, pwa := range a {
		inB := false
		for _, pwb := range b {
			if pwa.Equal(pwb) {
				inB = true
				break
			}
		}
		if !inB {
			ret = append(ret, pwa)
		}
	}
	return ret
}

type podWatchTaskList struct {
	watcherTaskList
	setup    []PodWatch
	teardown []PodWatch
}

func (w *PodWatcher) diff(ctx context.Context, st store.RStore) podWatchTaskList {
	state := st.RLockState()
	defer st.RUnlockState()

	w.mu.RLock()
	defer w.mu.RUnlock()

	taskList := createWatcherTaskList(state, w.knownDeployedUIDs)
	var neededWatches []PodWatch
	for _, mt := range state.Targets() {
		for _, ls := range mt.Manifest.K8sTarget().ExtraPodSelectors {
			if !ls.Empty() {
				neededWatches = append(neededWatches, PodWatch{name: mt.Manifest.Name, labels: ls})
			}
		}
	}
	if taskList.needsWatch {
		neededWatches = append(neededWatches, PodWatch{labels: k8s.ManagedByTiltSelector()})
	}

	return podWatchTaskList{
		watcherTaskList: taskList,
		setup:           subtract(neededWatches, w.watches),
		teardown:        subtract(w.watches, neededWatches),
	}
}

func (w *PodWatcher) OnChange(ctx context.Context, st store.RStore) {
	taskList := w.diff(ctx, st)

	for _, pw := range taskList.setup {
		ctx, cancel := context.WithCancel(ctx)
		pw.cancel = cancel
		w.addWatch(pw)
		ch, err := w.kCli.WatchPods(ctx, pw.labels)
		if err != nil {
			err = errors.Wrap(err, "Error watching pods. Are you connected to kubernetes?\n")
			st.Dispatch(store.NewErrorAction(err))
			return
		}
		go w.dispatchPodChangesLoop(ctx, ch, st)
	}

	for _, pw := range taskList.teardown {
		pw.cancel()
		w.removeWatch(pw)
	}

	if len(taskList.newUIDs) > 0 {
		w.setupNewUIDs(ctx, st, taskList.newUIDs)
	}
}

func (w *PodWatcher) addWatch(pw PodWatch) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.watches = append(w.watches, pw)
}

func (w *PodWatcher) removeWatch(toRemove PodWatch) {
	w.mu.Lock()
	defer w.mu.Unlock()
	oldWatches := append([]PodWatch{}, w.watches...)
	w.watches = nil
	for _, e := range oldWatches {
		if !e.Equal(toRemove) {
			w.watches = append(w.watches, e)
		}
	}
}

// When new UIDs are deployed, go through all our known pods and dispatch
// new actions. This handles the case where we get the Pod change event
// before the deploy id shows up in the manifest, which is way more common than
// you would think.
func (w *PodWatcher) setupNewUIDs(ctx context.Context, st store.RStore, newUIDs map[types.UID]model.ManifestName) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for uid, mn := range newUIDs {
		w.knownDeployedUIDs[uid] = mn

		pod, ok := w.knownPods[uid]
		if ok {
			st.Dispatch(NewPodChangeAction(pod, mn, uid))
			continue
		}

		descendants := w.knownDescendentPodUIDs[uid]
		for podUID := range descendants {
			pod, ok := w.knownPods[podUID]
			if ok {
				st.Dispatch(NewPodChangeAction(pod, mn, uid))
			}
		}
	}
}

func (w *PodWatcher) upsertPod(pod *v1.Pod) {
	w.mu.Lock()
	defer w.mu.Unlock()

	uid := pod.UID
	w.knownPods[uid] = pod
}

// Check to see if this pod corresponds to any of our manifests.
//
// Currently, we do this by comparing the pod UID and its owner UIDs against
// what we've deployed to the cluster. Returns the ManifestName and the UID that
// it matched against.
//
// If the pod doesn't match an existing deployed resource, keep it in local
// state, so we can match it later if the owner UID shows up.
func (w *PodWatcher) triagePodTree(pod *v1.Pod, objTree k8s.ObjectRefTree) (model.ManifestName, types.UID) {
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
			set = store.NewUIDSet()
			w.knownDescendentPodUIDs[ownerUID] = set
		}
		set.Add(uid)
	}

	// Find the manifest name
	for _, ownerUID := range objTree.UIDs() {
		mn, ok := w.knownDeployedUIDs[ownerUID]
		if ok {
			return mn, ownerUID
		}
	}

	// If we can't find the manifest based on owner, check to see if the pod any
	// of the manifest-specific pod selectors.
	//
	// NOTE(nick): This code might be totally obsolete now that we triage
	// pods by owner UID. It's meant to handle CRDs, but most CRDs should
	// set owner reference appropriately.
	podLabels := labels.Set(pod.ObjectMeta.GetLabels())
	for _, watch := range w.watches {
		if watch.name == "" {
			continue
		}

		if watch.labels.Matches(podLabels) {
			return watch.name, ""
		}
	}
	return "", ""
}

func (w *PodWatcher) dispatchPodChange(ctx context.Context, pod *v1.Pod, st store.RStore) {
	objTree, err := w.ownerFetcher.OwnerTreeOf(ctx, k8s.NewK8sEntity(pod))
	if err != nil {
		logger.Get(ctx).Infof("Handling pod update (%q): %v", pod.Name, err)
		return
	}

	mn, ancestorUID := w.triagePodTree(pod, objTree)
	if mn == "" {
		return
	}

	w.mu.Lock()
	freshPod, ok := w.knownPods[pod.UID]
	if ok {
		st.Dispatch(NewPodChangeAction(freshPod, mn, ancestorUID))
	}
	w.mu.Unlock()
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

// copied from https://github.com/kubernetes/kubernetes/blob/aedeccda9562b9effe026bb02c8d3c539fc7bb77/pkg/kubectl/resource_printer.go#L692-L764
// to match the status column of `kubectl get pods`
func PodStatusToString(pod v1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	for i, container := range pod.Status.InitContainerStatuses {
		state := container.State

		switch {
		case state.Terminated != nil && state.Terminated.ExitCode == 0:
			continue
		case state.Terminated != nil:
			// initialization is failed
			if len(state.Terminated.Reason) == 0 {
				if state.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", state.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", state.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + state.Terminated.Reason
			}
		case state.Waiting != nil && len(state.Waiting.Reason) > 0 && state.Waiting.Reason != "PodInitializing":
			reason = "Init:" + state.Waiting.Reason
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
		}
		break
	}

	if isPodStillInitializing(pod) {
		return reason
	}

	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]
		state := container.State

		if state.Waiting != nil && state.Waiting.Reason != "" {
			reason = state.Waiting.Reason
		} else if state.Terminated != nil && state.Terminated.Reason != "" {
			reason = state.Terminated.Reason
		} else if state.Terminated != nil && state.Terminated.Reason == "" {
			if state.Terminated.Signal != 0 {
				reason = fmt.Sprintf("Signal:%d", state.Terminated.Signal)
			} else {
				reason = fmt.Sprintf("ExitCode:%d", state.Terminated.ExitCode)
			}
		}
	}

	return reason
}

// Pull out interesting error messages from the pod status
func PodStatusErrorMessages(pod v1.Pod) []string {
	result := []string{}
	if isPodStillInitializing(pod) {
		for _, container := range pod.Status.InitContainerStatuses {
			result = append(result, containerStatusErrorMessages(container)...)
		}
	}
	for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
		container := pod.Status.ContainerStatuses[i]
		result = append(result, containerStatusErrorMessages(container)...)
	}
	return result
}

func containerStatusErrorMessages(container v1.ContainerStatus) []string {
	result := []string{}
	state := container.State
	if state.Waiting != nil {
		lastState := container.LastTerminationState
		if lastState.Terminated != nil &&
			lastState.Terminated.ExitCode != 0 &&
			lastState.Terminated.Message != "" {
			result = append(result, lastState.Terminated.Message)
		}

		// If we're in CrashLoopBackOff mode, also include the error message
		// so we know when the pod will get rescheduled.
		if state.Waiting.Message != "" && state.Waiting.Reason == "CrashLoopBackOff" {
			result = append(result, state.Waiting.Message)
		}
	} else if state.Terminated != nil &&
		state.Terminated.ExitCode != 0 &&
		state.Terminated.Message != "" {
		result = append(result, state.Terminated.Message)
	}

	return result
}

func isPodStillInitializing(pod v1.Pod) bool {
	for _, container := range pod.Status.InitContainerStatuses {
		state := container.State
		isFinished := state.Terminated != nil && state.Terminated.ExitCode == 0
		if !isFinished {
			return true
		}
	}
	return false
}
