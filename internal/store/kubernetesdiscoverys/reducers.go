package kubernetesdiscoverys

import (
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleKubernetesDiscoveryUpsertAction(state *store.EngineState, action KubernetesDiscoveryUpsertAction) {
	n := action.KubernetesDiscovery.Name
	oldState := state.KubernetesDiscoverys[n]
	state.KubernetesDiscoverys[n] = action.KubernetesDiscovery

	// We only refresh when the K8sDiscovery is changed.
	//
	// This is really only needed for tests - we have tests that wait until we've
	// reached a steady state, then change some fields on EngineState.
	//
	// K8s controllers assume everything is idempotent, and will wipe out our changes
	// later with duplicate events.
	isChanged := oldState == nil ||
		!apicmp.DeepEqual(oldState.Status, action.KubernetesDiscovery.Status) ||
		!apicmp.DeepEqual(oldState.Spec, action.KubernetesDiscovery.Spec)
	if isChanged {
		RefreshKubernetesResource(state, n)
	}
}

func HandleKubernetesDiscoveryDeleteAction(state *store.EngineState, action KubernetesDiscoveryDeleteAction) {
	oldState := state.KubernetesDiscoverys[action.Name]
	delete(state.KubernetesDiscoverys, action.Name)

	isChanged := oldState != nil
	if isChanged {
		RefreshKubernetesResource(state, action.Name)
	}
}

func filterForResource(state *store.EngineState, name string) (*k8sconv.KubernetesApplyFilter, error) {
	a := state.KubernetesApplys[name]
	if a == nil {
		return nil, nil
	}

	// if the yaml matches the existing resource, use its filter to save re-parsing
	// (https://github.com/tilt-dev/tilt/issues/5837)
	if prevResource, ok := state.KubernetesResources[name]; ok {
		if a.Status.ResultYAML == prevResource.ApplyStatus.ResultYAML {
			return prevResource.ApplyFilter, nil
		}
	}

	return k8sconv.NewKubernetesApplyFilter(a.Status.ResultYAML)
}

func RefreshKubernetesResource(state *store.EngineState, name string) {
	var aStatus *v1alpha1.KubernetesApplyStatus
	a := state.KubernetesApplys[name]
	if a != nil {
		aStatus = &(a.Status)
	}

	d := state.KubernetesDiscoverys[name]
	filter, err := filterForResource(state, name)
	if err != nil {
		return
	}
	r := k8sconv.NewKubernetesResourceWithFilter(d, aStatus, filter)
	state.KubernetesResources[name] = r

	if a != nil {
		mn := model.ManifestName(a.Annotations[v1alpha1.AnnotationManifest])
		ms, ok := state.ManifestState(mn)
		if ok {
			krs := ms.K8sRuntimeState()

			if d == nil {
				// if the KubernetesDiscovery goes away, we no longer know about any pods
				krs.FilteredPods = nil
				ms.RuntimeState = krs
				return
			}

			krs.FilteredPods = r.FilteredPods
			krs.Conditions = r.ApplyStatus.Conditions

			isReadyOrSucceeded := false
			if len(r.FilteredPods) != 0 {
				for _, pod := range r.FilteredPods {
					if krs.PodReadinessMode == model.PodReadinessSucceeded {
						// for jobs, we don't care about whether it's ready, only whether it's succeeded
						isReadyOrSucceeded = pod.Phase == string(v1.PodSucceeded)
					} else {
						isReadyOrSucceeded = len(pod.Containers) != 0 && store.AllPodContainersReady(pod)
					}
				}
			} else {
				isReadyOrSucceeded = meta.IsStatusConditionTrue(r.ApplyStatus.Conditions, v1alpha1.ApplyConditionJobComplete)
			}

			if isReadyOrSucceeded {
				// NOTE(nick): It doesn't seem right to update this timestamp everytime
				// we get a new event, but it's what the old code did.
				krs.LastReadyOrSucceededTime = time.Now()
			}

			ms.RuntimeState = krs
		}
	}
}
