package engine

import (
	"context"
	"fmt"
	"strconv"

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
	podInfo.Deleting = pod.DeletionTimestamp != nil
	podInfo.Phase = pod.Status.Phase
	podInfo.Status = podStatusToString(*pod)
	podInfo.StatusMessages = podStatusErrorMessages(*pod)

	defer prunePods(ms)

	// Check if the container is ready.
	var cStatus v1.ContainerStatus
	var err error
	if len(manifest.ImageTargets) > 0 {
		// Get status of (first) container matching (an) image we built for this manifest.
		for _, iTarget := range manifest.ImageTargets {
			cStatus, err = k8s.ContainerMatching(pod, container.NameSelector(iTarget.DeploymentRef))
			if err != nil {
				logger.Get(ctx).Debugf("Error matching container: %v", err)
				return
			}
			if cStatus.Name != "" {
				break
			}
		}
	} else {
		// We didn't build images for this manifest so we have no good way of figuring
		// out which container(s) we care about; for now, take the first.
		if len(pod.Status.ContainerStatuses) > 0 {
			cStatus = pod.Status.ContainerStatuses[0]
		}

	}

	if cStatus.Name == "" {
		return
	}

	populateContainerStatus(ctx, manifest, podInfo, pod, cStatus)
	checkForPodCrash(ctx, state, ms, *podInfo)

	if int(cStatus.RestartCount) > podInfo.ContainerRestarts {
		ms.CrashLog = podInfo.CurrentLog
		podInfo.CurrentLog = model.Log{}
	}
	podInfo.ContainerRestarts = int(cStatus.RestartCount)
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

// Fill in container fields on the pod state.
func populateContainerStatus(ctx context.Context, manifest model.Manifest, podInfo *store.Pod, pod *v1.Pod, cStatus v1.ContainerStatus) {
	cName := k8s.ContainerNameFromContainerStatus(cStatus)

	cID, err := k8s.ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
		return
	}

	cRef, err := container.ParseNamed(cStatus.Image)
	if err != nil {
		logger.Get(ctx).Debugf("Error parsing container image ID: %v", err)
		return
	}

	ports := make([]int32, 0)
	cSpec := k8s.ContainerSpecOf(pod, cStatus)
	for _, cPort := range cSpec.Ports {
		ports = append(ports, cPort.ContainerPort)
	}

	forwards := PopulatePortForwards(manifest, *podInfo)
	if len(forwards) < len(manifest.K8sTarget().PortForwards) {
		logger.Get(ctx).Infof(
			"WARNING: Resource %s is using port forwards, but no container ports on pod %s",
			manifest.Name, podInfo.PodID)
	}

	// ~~ If this container is already on the pod, replace it
	// ~~ this interim code assumes one pod per container, so just make it so
	c := store.Container{
		Name:     cName,
		ID:       cID,
		Ports:    ports,
		Ready:    cStatus.Ready,
		ImageRef: cRef,
	}
	podInfo.Containers = []store.Container{c}

	// HACK(maia): Go through ALL containers (except tilt-synclet), grab minimum info we need
	// to stream logs from them.
	var cInfos []store.ContainerInfo
	for _, cStat := range pod.Status.ContainerStatuses {
		if cStat.Name == sidecar.SyncletContainerName {
			// We don't want logs for the Tilt synclet.
			continue
		}

		cID, err := k8s.ContainerIDFromContainerStatus(cStat)
		if err != nil {
			logger.Get(ctx).Debugf("Error parsing container ID: %v", err)
			return
		}

		cInfos = append(cInfos, store.ContainerInfo{
			ID:   cID,
			Name: k8s.ContainerNameFromContainerStatus(cStat),
		})
	}
	podInfo.ContainerInfos = cInfos
}

func checkForPodCrash(ctx context.Context, state *store.EngineState, ms *store.ManifestState, podInfo store.Pod) {
	if ms.NeedsRebuildFromCrash {
		// We're already aware the pod is crashing.
		return
	}

	if ms.ExpectedContainerID == "" || ms.ExpectedContainerID == podInfo.ContainerID() {
		// The pod is what we expect it to be.
		return
	}

	// The pod isn't what we expect!
	ms.CrashLog = podInfo.CurrentLog
	ms.NeedsRebuildFromCrash = true
	ms.ExpectedContainerID = ""
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
	// Continue pruning until we have 1 pod.
	for ms.PodSet.Len() > 1 {
		bestPod := ms.MostRecentPod()

		for key, pod := range ms.PodSet.Pods {
			// Always remove pods that were manually deleted.
			if pod.Deleting {
				delete(ms.PodSet.Pods, key)
				break
			}

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
