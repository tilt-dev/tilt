package k8swatch

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/portforward"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleKubernetesDiscoveryCreateAction(_ context.Context, state *store.EngineState, a KubernetesDiscoveryCreateAction) {
	key := apis.Key(a.KubernetesDiscovery)
	if _, ok := state.KubernetesDiscoveries[key]; !ok {
		state.KubernetesDiscoveries[key] = a.KubernetesDiscovery
	}
}

func HandleKubernetesDiscoveryUpdateAction(ctx context.Context, state *store.EngineState, a KubernetesDiscoveryUpdateAction) {
	key := apis.Key(a.KubernetesDiscovery)
	if _, ok := state.KubernetesDiscoveries[key]; ok {
		state.KubernetesDiscoveries[key] = a.KubernetesDiscovery
	}
	UpdateK8sRuntimeState(ctx, state, key)
}

func HandleKubernetesDiscoveryUpdateStatusAction(ctx context.Context, state *store.EngineState, a KubernetesDiscoveryUpdateStatusAction) {
	key := apis.KeyFromMeta(*a.ObjectMeta)
	if _, ok := state.KubernetesDiscoveries[key]; ok {
		state.KubernetesDiscoveries[key].Status = *a.Status
	}
	UpdateK8sRuntimeState(ctx, state, key)
}

func HandleKubernetesDiscoveryDeleteAction(_ context.Context, state *store.EngineState, a KubernetesDiscoveryDeleteAction) {
	delete(state.KubernetesDiscoveries, a.Name)
}

func manifestAndKubernetesDiscoveryStatus(key types.NamespacedName, state *store.EngineState) (model.ManifestName, *v1alpha1.KubernetesDiscoveryStatus) {
	kd := state.KubernetesDiscoveries[key]
	if kd == nil {
		return "", nil
	}

	mn := model.ManifestName(kd.Annotations[v1alpha1.AnnotationManifest])
	if mn == "" {
		return "", nil
	}

	return mn, kd.Status.DeepCopy()
}

func UpdateK8sRuntimeState(ctx context.Context, state *store.EngineState, key types.NamespacedName) {
	mn, status := manifestAndKubernetesDiscoveryStatus(key, state)
	if mn == "" {
		// this spec isn't for a manifest, so there's no manifest runtime state to update
		return
	}

	mt := state.ManifestTargets[mn]
	if mt == nil {
		// This is OK. The user could have edited the manifest recently.
		return
	}
	ms := mt.State
	krs := ms.K8sRuntimeState()

	anyPodsUpdated := false
	seenPods := make(map[k8s.PodID]bool)
	for i := range status.Pods {
		pod := &status.Pods[i]
		seenPods[k8s.PodID(pod.Name)] = true
		if !maybeUpdateStateForPod(ms, pod) {
			continue
		}
		anyPodsUpdated = true
		fwdsValid := portforward.PortForwardsAreValid(mt.Manifest, *pod)
		if !fwdsValid {
			logger.Get(ctx).Warnf(
				"Resource %s is using port forwards, but no container ports on pod %s",
				mn, pod.Name)
		}
	}

	for podID := range krs.Pods {
		if !seenPods[podID] {
			delete(krs.Pods, podID)
		}
	}

	prunePods(ms)

	if anyPodsUpdated {
		CheckForContainerCrash(state, mt)
	}
}

func maybeUpdateStateForPod(ms *store.ManifestState, pod *v1alpha1.Pod) bool {
	podID := k8s.PodID(pod.Name)
	runtime := ms.K8sRuntimeState()

	if !runtime.HasOKPodTemplateSpecHash(pod) {
		// If this is from an outdated deploy but the pod is still being tracked, we
		// will still update it; if it's outdated and untracked, just ignore
		if _, alreadyTracked := runtime.Pods[podID]; alreadyTracked {
			runtime.Pods[podID] = pod
			return true
		}
		return false
	}

	// (Re-)initialize state if we haven't seen pods for this ancestor yet
	matchedAncestorUID := types.UID(pod.AncestorUID)
	isAncestorMatch := matchedAncestorUID != ""
	if runtime.PodAncestorUID == "" ||
		(isAncestorMatch && runtime.PodAncestorUID != matchedAncestorUID) {

		// Track a new ancestor ID, and delete all existing tracked pods.
		runtime.Pods = make(map[k8s.PodID]*v1alpha1.Pod)
		runtime.PodAncestorUID = matchedAncestorUID
	}

	if existing := runtime.Pods[podID]; existing != nil && equality.Semantic.DeepEqual(existing, pod) {
		return false
	}

	// Track the new version
	runtime.Pods[podID] = pod

	if len(pod.Containers) != 0 && (store.AllPodContainersReady(*pod) || pod.Phase == string(v1.PodSucceeded)) {
		runtime.LastReadyOrSucceededTime = time.Now()
	}

	ms.RuntimeState = runtime
	return true
}

func CheckForContainerCrash(state *store.EngineState, mt *store.ManifestTarget) {
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
	state.LogStore.Append(le, state.Secrets)
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
