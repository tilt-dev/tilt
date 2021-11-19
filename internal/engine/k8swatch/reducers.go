package k8swatch

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleKubernetesDiscoveryUpdateStatusAction(ctx context.Context, state *store.EngineState, a KubernetesDiscoveryUpdateStatusAction) {
	UpdateK8sRuntimeState(ctx, state, a.ObjectMeta, a.Status)
}

func UpdateK8sRuntimeState(ctx context.Context, state *store.EngineState, objMeta *metav1.ObjectMeta, status *v1alpha1.KubernetesDiscoveryStatus) {
	mn := model.ManifestName(objMeta.Annotations[v1alpha1.AnnotationManifest])
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
	}

	for podID := range krs.Pods {
		if !seenPods[podID] {
			delete(krs.Pods, podID)
		}
	}

	prunePods(ms)

	if anyPodsUpdated {
		liveupdates.CheckForContainerCrash(state, mn.String())
	}
}

func maybeUpdateStateForPod(ms *store.ManifestState, pod *v1alpha1.Pod) bool {
	podID := k8s.PodID(pod.Name)
	runtime := ms.K8sRuntimeState()

	if !k8sconv.HasOKPodTemplateSpecHash(pod, runtime.ApplyFilter) {
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

	existing := runtime.Pods[podID]
	if existing != nil && equality.Semantic.DeepEqual(existing, pod) {
		return false
	}

	// Track the new version
	runtime.Pods[podID] = pod

	isReadyOrSucceeded := false
	if ms.K8sRuntimeState().PodReadinessMode == model.PodReadinessSucceeded {
		// for jobs, we don't care about whether it's ready, only whether it's succeeded
		isReadyOrSucceeded = pod.Phase == string(v1.PodSucceeded)
	} else {
		isReadyOrSucceeded = len(pod.Containers) != 0 && store.AllPodContainersReady(*pod)
	}
	if isReadyOrSucceeded {
		runtime.LastReadyOrSucceededTime = time.Now()
	}

	ms.RuntimeState = runtime
	return true
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
