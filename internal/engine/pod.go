package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/k8swatch"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func handlePodDeleteAction(_ context.Context, state *store.EngineState, action k8swatch.PodDeleteAction) {
	// PodDeleteActions only have the pod id. We don't have a good way to tie them back to their ancestors.
	// So just brute-force it.
	for _, target := range state.ManifestTargets {
		ms := target.State
		runtime := ms.K8sRuntimeState()
		delete(runtime.Pods, action.PodID)
	}
}

func handlePodChangeAction(ctx context.Context, state *store.EngineState, action k8swatch.PodChangeAction) {
	mt := matchPodChangeToManifest(state, action)
	if mt == nil {
		return
	}

	pod := action.Pod
	podID := k8s.PodID(pod.Name)
	spanID := k8sconv.SpanIDForPod(podID)
	ms := mt.State
	krs := ms.K8sRuntimeState()
	manifest := mt.Manifest

	var existing *v1alpha1.Pod
	isCurrentDeploy := krs.HasOKPodTemplateSpecHash(pod)
	if isCurrentDeploy {
		// Only attach a new pod to the runtime state if it's from the current deploy;
		// if it's from an old deploy/an old Tilt run, we don't want to be checking it
		// for status etc.
		existing = trackPod(ms, action)
	} else {
		// If this is from an outdated deploy but the pod is still being tracked, we
		// will still update it; if it's outdated and untracked, just ignore
		existing = krs.Pods[podID]
		if existing == nil {
			return
		}
		krs.Pods[podID] = pod
	}

	prunePods(ms)

	if existing != nil {
		names := restartedContainerNames(existing.InitContainers, pod.InitContainers)
		for _, name := range names {
			s := fmt.Sprintf("Detected container restart. Pod: %s. Container: %s.", existing.Name, name)
			handleLogAction(state, store.NewLogAction(manifest.Name, spanID, logger.WarnLvl, nil, []byte(s)))
		}
	}

	if existing != nil {
		names := restartedContainerNames(existing.Containers, pod.Containers)
		for _, name := range names {
			s := fmt.Sprintf("Detected container restart. Pod: %s. Container: %s.", existing.Name, name)
			handleLogAction(state, store.NewLogAction(manifest.Name, spanID, logger.WarnLvl, nil, []byte(s)))
		}
	}

	if existing == nil {
		// This is the first time we've seen this pod.
		// Ignore any restarts that happened before Tilt saw it.
		//
		// This can happen when the image was deployed on a previous
		// Tilt run, so we're just attaching to an existing pod
		// with some old history.
		pod.BaselineRestartCount = store.AllPodContainerRestarts(*pod)
	}

	if len(pod.Containers) == 0 {
		// not enough info to do anything else
		return
	}

	if store.AllPodContainersReady(*pod) || pod.Phase == string(v1.PodSucceeded) {
		runtime := ms.K8sRuntimeState()
		runtime.LastReadyOrSucceededTime = time.Now()
		ms.RuntimeState = runtime
	}

	fwdsValid := portforward.PortForwardsAreValid(manifest, *pod)
	if !fwdsValid {
		logger.Get(ctx).Warnf(
			"Resource %s is using port forwards, but no container ports on pod %s",
			manifest.Name, pod.Name)
	}
	checkForContainerCrash(state, mt)
}

func restartedContainerNames(existingContainers []v1alpha1.Container, newContainers []v1alpha1.Container) []container.Name {
	result := []container.Name{}
	for i, c := range newContainers {
		if i >= len(existingContainers) {
			break
		}

		existing := existingContainers[i]
		if existing.Name != c.Name {
			continue
		}

		if c.Restarts > existing.Restarts {
			result = append(result, container.Name(c.Name))
		}
	}
	return result
}

// Find the ManifestTarget for the PodChangeAction,
// and confirm that it matches what we've deployed.
func matchPodChangeToManifest(state *store.EngineState, action k8swatch.PodChangeAction) *store.ManifestTarget {
	manifestName := action.ManifestName
	matchedAncestorUID := action.MatchedAncestorUID
	mt, ok := state.ManifestTargets[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil
	}

	ms := mt.State
	runtime := ms.K8sRuntimeState()

	// If the event has an ancestor UID attached, but that ancestor isn't in the
	// deployed UID set anymore, we can ignore it.
	isAncestorMatched := matchedAncestorUID != ""
	if isAncestorMatched && !runtime.DeployedEntities.ContainsUID(matchedAncestorUID) {
		return nil
	}
	return mt
}

// trackPod adds a Pod to the RuntimeState for a resource.
//
// If there are currently tracked Pods and the new Pod's ancestor UID differs from theirs,
// the tracked Pods will be cleared and the ancestor UID updated. Otherwise, the Pod will
// be upserted and the prior version returned.
//
// A mutable reference to the Pod to be tracked is stored so that further changes to it
// can be made based on the diff from the returned prior version. The caller should *not*
// hold their reference beyond the action handler. This is a temporary situation to ease
// transition of this data to an API server reconciler; currently, the reducer here is
// both handling the tracking as well as using diff to derive things like container restarts.
func trackPod(ms *store.ManifestState, action k8swatch.PodChangeAction) *v1alpha1.Pod {
	pod := action.Pod
	podID := k8s.PodID(pod.Name)
	runtime := ms.K8sRuntimeState()

	// (Re-)initialize state if we haven't seen pods for this ancestor yet
	matchedAncestorUID := action.MatchedAncestorUID
	isAncestorMatch := matchedAncestorUID != ""
	if runtime.PodAncestorUID == "" ||
		(isAncestorMatch && runtime.PodAncestorUID != matchedAncestorUID) {

		// Track a new ancestor ID, and delete all existing tracked pods.
		runtime.Pods = make(map[k8s.PodID]*v1alpha1.Pod)
		runtime.PodAncestorUID = matchedAncestorUID
		ms.RuntimeState = runtime
	}

	// Return the existing (if any) and track the new version
	existing := runtime.Pods[podID]
	runtime.Pods[podID] = pod
	return existing
}

func checkForContainerCrash(state *store.EngineState, mt *store.ManifestTarget) {
	ms := mt.State
	if ms.NeedsRebuildFromCrash {
		// We're already aware the pod is crashing.
		return
	}

	runningContainers := store.AllRunningContainers(mt)
	hitList := make(map[container.ID]bool, len(ms.LiveUpdatedContainerIDs))
	for cID := range ms.LiveUpdatedContainerIDs {
		hitList[cID] = true
	}
	for _, c := range runningContainers {
		delete(hitList, c.ContainerID)
	}

	if len(hitList) == 0 {
		// The pod is what we expect it to be.
		return
	}

	// The pod isn't what we expect!
	ms.NeedsRebuildFromCrash = true
	ms.LiveUpdatedContainerIDs = container.NewIDSet()
	msg := fmt.Sprintf("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Name)
	le := store.NewLogAction(ms.Name, ms.LastBuild().SpanID, logger.WarnLvl, nil, []byte(msg+"\n"))
	handleLogAction(state, le)
}

// If there's more than one pod, prune the deleting/dead ones so
// that they don't clutter the output.
func prunePods(ms *store.ManifestState) {
	// Always remove pods that were manually deleted.
	runtime := ms.K8sRuntimeState()
	for key, pod := range runtime.Pods {
		if pod.Deleting {
			delete(runtime.Pods, key)
		}
	}
	// Continue pruning until we have 1 pod.
	for runtime.PodLen() > 1 {
		bestPod := ms.MostRecentPod()

		for key, pod := range runtime.Pods {
			// Remove terminated pods if they aren't the most recent one.
			isDead := pod.Phase == string(v1.PodSucceeded) || pod.Phase == string(v1.PodFailed)
			if isDead && pod.Name != bestPod.Name {
				delete(runtime.Pods, key)
				break
			}
		}

		// found nothing to delete, break out
		// NOTE(dmiller): above comment is probably erroneous, but disabling this check because I'm not sure if this is safe to change
		// original static analysis error:
		// SA4004: the surrounding loop is unconditionally terminated (staticcheck)
		//nolint:staticcheck
		return
	}
}

func handlePodResetRestartsAction(state *store.EngineState, action store.PodResetRestartsAction) {
	ms, ok := state.ManifestState(action.ManifestName)
	if !ok {
		return
	}

	runtime := ms.K8sRuntimeState()
	podInfo, ok := runtime.Pods[action.PodID]
	if !ok {
		return
	}

	// We have to be careful here because the pod might have restarted
	// since the action was created.
	delta := store.VisiblePodContainerRestarts(*podInfo) - action.VisibleRestarts
	podInfo.BaselineRestartCount = store.AllPodContainerRestarts(*podInfo) - delta
}
