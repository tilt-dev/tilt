package engine

import (
	"context"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
)

func handlePodChangeAction(ctx context.Context, state *store.EngineState, pod *v1.Pod) {
	mt, podInfo := ensureManifestTargetWithPod(state, pod)
	if mt == nil || podInfo == nil {
		return
	}

	ms := mt.State
	manifest := mt.Manifest
	podID := k8s.PodIDFromPod(pod)
	if podInfo.PodID != podID {
		// This is an event from an old pod.
		return
	}

	// Update the status
	podInfo.Deleting = pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero()
	podInfo.Phase = pod.Status.Phase
	podInfo.Status = podStatusToString(*pod)
	podInfo.StatusMessages = podStatusErrorMessages(*pod)

	prunePods(ms)

	oldRestartTotal := podInfo.AllContainerRestarts()
	podInfo.Containers = podContainers(ctx, pod)

	if len(podInfo.Containers) == 0 {
		// not enough info to do anything else
		return
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

// Get a pointer to a mutable manifest state,
// ensuring that some Pod exists on the state.
//
// Intended as a helper for pod-mutating events.
func ensureManifestTargetWithPod(state *store.EngineState, pod *v1.Pod) (*store.ManifestTarget, *store.Pod) {
	manifestName := model.ManifestName(pod.ObjectMeta.Labels[k8s.ManifestNameLabel])
	if manifestName == "" {
		// if there's no ManifestNameLabel, then maybe it matches some manifest's ExtraPodSelectors
		for _, m := range state.Manifests() {
			if m.IsK8s() {
				for _, lps := range m.K8sTarget().ExtraPodSelectors {
					if lps.Matches(labels.Set(pod.ObjectMeta.GetLabels())) {
						manifestName = m.Name
						break
					}
				}
			}
		}
	}

	mt, ok := state.ManifestTargets[manifestName]
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return nil, nil
	}

	ms := mt.State

	deployID := ms.DeployID
	if podDeployID, ok := pod.ObjectMeta.Labels[k8s.TiltDeployIDLabel]; ok {
		if pdID, err := strconv.Atoi(podDeployID); err != nil || pdID != int(deployID) {
			return nil, nil
		}
	}

	podID := k8s.PodIDFromPod(pod)
	startedAt := pod.CreationTimestamp.Time
	status := podStatusToString(*pod)
	ns := k8s.NamespaceFromPod(pod)
	hasSynclet := sidecar.PodSpecContainsSynclet(pod.Spec)

	// CASE 1: We don't have a set of pods for this DeployID yet
	if ms.PodSet.DeployID == 0 || ms.PodSet.DeployID != deployID {
		ms.PodSet = store.PodSet{
			DeployID: deployID,
			Pods:     make(map[k8s.PodID]*store.Pod),
		}
		ms.PodSet.Pods[podID] = &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		return mt, ms.PodSet.Pods[podID]
	}

	podInfo, ok := ms.PodSet.Pods[podID]
	if !ok {
		// CASE 2: We have a set of pods for this DeployID, but not this particular pod -- record it
		podInfo = &store.Pod{
			PodID:      podID,
			StartedAt:  startedAt,
			Status:     status,
			Namespace:  ns,
			HasSynclet: hasSynclet,
		}
		ms.PodSet.Pods[podID] = podInfo
	}

	// CASE 3: This pod is already in the PodSet, nothing to do.
	return mt, podInfo
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
		ms.BuildHistory[0].Log = model.AppendLog(ms.BuildHistory[0].Log, le, state.LogTimestamps, "")
	}
	ms.CurrentBuild.Log = model.AppendLog(ms.CurrentBuild.Log, le, state.LogTimestamps, "")
	handleLogAction(state, le)
}

// If there's more than one pod, prune the deleting/dead ones so
// that they don't clutter the output.
func prunePods(ms *store.ManifestState) {
	// Always remove pods that were manually deleted.
	for key, pod := range ms.PodSet.Pods {
		if pod.Deleting {
			delete(ms.PodSet.Pods, key)
		}
	}
	// Continue pruning until we have 1 pod.
	for ms.PodSet.Len() > 1 {
		bestPod := ms.MostRecentPod()

		for key, pod := range ms.PodSet.Pods {
			// Remove terminated pods if they aren't the most recent one.
			isDead := pod.Phase == v1.PodSucceeded || pod.Phase == v1.PodFailed
			if isDead && pod.PodID != bestPod.PodID {
				delete(ms.PodSet.Pods, key)
				break
			}
		}

		// found nothing to delete, break out
		return
	}
}

func handlePodLogAction(state *store.EngineState, action PodLogAction) {
	manifestName := action.Source()
	ms, ok := state.ManifestState(manifestName)
	if !ok {
		// This is OK. The user could have edited the manifest recently.
		return
	}

	podID := action.PodID
	if !ms.PodSet.ContainsID(podID) {
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

	podInfo := ms.PodSet.Pods[podID]
	podInfo.CurrentLog = model.AppendLog(podInfo.CurrentLog, action, state.LogTimestamps, "")
}
