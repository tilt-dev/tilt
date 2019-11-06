package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/engine/k8swatch"
	"github.com/windmilleng/tilt/internal/engine/runtimelog"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

func handlePodChangeAction(ctx context.Context, state *store.EngineState, action k8swatch.PodChangeAction) {
	mt := matchPodChangeToManifest(state, action)
	if mt == nil {
		return
	}

	pod := action.Pod
	ms := mt.State
	manifest := mt.Manifest
	podInfo, isNew := maybeTrackPod(ms, action)
	podID := k8s.PodIDFromPod(pod)
	if podInfo.PodID != podID {
		// This is an event from an old pod.
		return
	}

	// Update the status
	podInfo.Deleting = pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero()
	podInfo.Phase = pod.Status.Phase
	podInfo.Status = k8swatch.PodStatusToString(*pod)
	podInfo.StatusMessages = k8swatch.PodStatusErrorMessages(*pod)

	prunePods(ms)

	oldRestartTotal := podInfo.AllContainerRestarts()
	podInfo.Containers = podContainers(ctx, pod)
	if isNew {
		// This is the first time we've seen this pod.
		// Ignore any restarts that happened before Tilt saw it.
		//
		// This can happen when the image was deployed on a previous
		// Tilt run, so we're just attaching to an existing pod
		// with some old history.
		podInfo.BaselineRestarts = podInfo.AllContainerRestarts()
	}

	if len(podInfo.Containers) == 0 {
		// not enough info to do anything else
		return
	}

	if podInfo.AllContainersReady() {
		runtime := ms.K8sRuntimeState()
		runtime.LastReadyTime = time.Now()
		ms.RuntimeState = runtime
	}

	fwdsValid := portForwardsAreValid(manifest, *podInfo)
	if !fwdsValid {
		logger.Get(ctx).Infof(
			"WARNING: Resource %s is using port forwards, but no container ports on pod %s",
			manifest.Name, podInfo.PodID)
	}
	checkForContainerCrash(ctx, state, mt)

	if oldRestartTotal < podInfo.AllContainerRestarts() {
		ms.CrashLog = podInfo.CurrentLog
		podInfo.CurrentLog = model.Log{}
	}
}

// Find the ManifestTarget for the PodChangeAction,
// and confirm that it matches what we've deployed.
func matchPodChangeToManifest(state *store.EngineState, action k8swatch.PodChangeAction) *store.ManifestTarget {
	manifestName := action.ManifestName
	ancestorUID := action.AncestorUID
	mt, ok := state.ManifestTargets[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil
	}

	ms := mt.State
	runtime := ms.GetOrCreateK8sRuntimeState()

	// If the event has an ancestor UID attached, but that ancestor isn't in the
	// deployed UID set anymore, we can ignore it.
	isAncestorMatch := ancestorUID != ""
	if isAncestorMatch && !runtime.DeployedUIDSet.Contains(ancestorUID) {
		return nil
	}
	return mt
}

// Checks the runtime state if we're already tracking this pod.
// If not, AND if the pod matches the current deploy, create a new tracking object.
// Returns a store.Pod that the caller can mutate, and true
// if this is the first time we've seen this pod.
func maybeTrackPod(ms *store.ManifestState, action k8swatch.PodChangeAction) (*store.Pod, bool) {
	pod := action.Pod
	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := k8swatch.PodStatusToString(*pod)
	ns := k8s.NamespaceFromPod(pod)
	hasSynclet := sidecar.PodSpecContainsSynclet(pod.Spec)
	runtime := ms.GetOrCreateK8sRuntimeState()
	currentDeploy := runtime.HasOKPodTemplateSpecHash(pod)

	// Case 1: We haven't seen pods for this ancestor yet.
	ancestorUID := action.AncestorUID
	isAncestorMatch := ancestorUID != ""
	if runtime.PodAncestorUID == "" ||
		(isAncestorMatch && runtime.PodAncestorUID != ancestorUID) {
		runtime.PodAncestorUID = ancestorUID
		runtime.Pods = make(map[k8s.PodID]*store.Pod)
		podInfo := &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		maybeAttachPodToManifest(ms, runtime, podInfo, currentDeploy)
		return podInfo, true
	}

	podInfo, ok := runtime.Pods[podID]
	if !ok {
		// CASE 2: We have a set of pods for this ancestor UID, but not this
		// particular pod -- record it
		podInfo = &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		maybeAttachPodToManifest(ms, runtime, podInfo, currentDeploy)
		return podInfo, true
	}

	// CASE 3: This pod is already in the PodSet, nothing to do.
	return podInfo, false
}

func maybeAttachPodToManifest(ms *store.ManifestState, runtime store.K8sRuntimeState,
	pod *store.Pod, currentDeploy bool) {
	// Only attach a new pod to the runtime state if it's from the current deploy;
	// if it's from an old deploy/an old Tilt run, we don't want to be checking it
	// for status etc.
	if currentDeploy {
		runtime.Pods[pod.PodID] = pod
		ms.RuntimeState = runtime
	}
}

// Convert a Kubernetes Pod into a list if simpler Container models to store in the engine state.
func podContainers(ctx context.Context, pod *v1.Pod) []store.Container {
	result := make([]store.Container, 0, len(pod.Status.ContainerStatuses))
	for _, cStatus := range pod.Status.ContainerStatuses {
		c, err := containerForStatus(ctx, pod, cStatus)
		if err != nil {
			logger.Get(ctx).Debugf(err.Error())
			continue
		}

		if !c.Empty() {
			result = append(result, c)
		}
	}
	return result
}

// Convert a Kubernetes Pod and ContainerStatus into a simpler Container model to store in the engine state.
func containerForStatus(ctx context.Context, pod *v1.Pod, cStatus v1.ContainerStatus) (store.Container, error) {
	if cStatus.Name == sidecar.SyncletContainerName {
		// We don't want logs, status, etc. for the Tilt synclet.
		return store.Container{}, nil
	}

	cName := k8s.ContainerNameFromContainerStatus(cStatus)

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		return store.Container{}, errors.Wrap(err, "Error parsing container ID")
	}

	cRef, err := container.ParseNamed(cStatus.Image)
	if err != nil {
		return store.Container{}, errors.Wrap(err, "Error parsing container image ID")

	}

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}

	return store.Container{
		Name:     cName,
		ID:       cID,
		Ports:    ports,
		Ready:    cStatus.Ready,
		ImageRef: cRef,
		Restarts: int(cStatus.RestartCount),
	}, nil
}

func checkForContainerCrash(ctx context.Context, state *store.EngineState, mt *store.ManifestTarget) {
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
	// TODO(nick): We should store the logs by container ID, and
	// only put the container that crashed in the CrashLog.
	ms.CrashLog = ms.MostRecentPod().CurrentLog
	ms.NeedsRebuildFromCrash = true
	ms.LiveUpdatedContainerIDs = container.NewIDSet()
	msg := fmt.Sprintf("Detected a container change for %s. We could be running stale code. Rebuilding and deploying a new image.", ms.Name)
	le := store.NewLogEvent(ms.Name, []byte(msg+"\n"))
	if len(ms.BuildHistory) > 0 {
		ms.BuildHistory[0].Log = model.AppendLog(ms.BuildHistory[0].Log, le, state.LogTimestamps, "", state.Secrets)
	}
	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, le, state.LogTimestamps, "", state.Secrets)
	handleLogAction(state, le)
}

// If there's more than one pod, prune the deleting/dead ones so
// that they don't clutter the output.
func prunePods(ms *store.ManifestState) {
	// Always remove pods that were manually deleted.
	runtime := ms.GetOrCreateK8sRuntimeState()
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
			isDead := pod.Phase == v1.PodSucceeded || pod.Phase == v1.PodFailed
			if isDead && pod.PodID != bestPod.PodID {
				delete(runtime.Pods, key)
				break
			}
		}

		// found nothing to delete, break out
		return
	}
}

func handlePodLogAction(state *store.EngineState, action runtimelog.PodLogAction) {
	manifestName := action.Source()
	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	podID := action.PodID
	runtime := ms.GetOrCreateK8sRuntimeState()
	if !runtime.ContainsID(podID) {
		// NOTE(nick): There are two cases where this could happen:
		// 1) Pod 1 died and kubernetes started Pod 2. What should we do with
		//    logs from Pod 1 that are still in the action queue?
		//    This is an open product question. A future HUD may aggregate
		//    logs across pod restarts.
		// 2) Due to race conditions, we got the logs for Pod 1 before
		//    we saw Pod 1 materialize on the Pod API. The best way to fix
		//    this would be to make PodLogManager a subscriber that only
		//    starts listening on logs once the pod has materialized.
		//    We may prioritize this higher or lower based on how often
		//    this happens in practice.
		return
	}

	podInfo := runtime.Pods[podID]
	podInfo.CurrentLog = model.AppendLog(podInfo.CurrentLog, action, state.LogTimestamps, "", state.Secrets)
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
	delta := podInfo.VisibleContainerRestarts() - action.VisibleRestarts
	podInfo.BaselineRestarts = podInfo.AllContainerRestarts() - delta
}
