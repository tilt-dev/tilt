package buildcontrol

import (
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

// NOTE(maia): we eventually want to move the BuildController into its own package
// (as we do with all subscribers), but for now, just move the underlying functions
// so they can be used from elsewhere.

// Algorithm to choose a manifest to build next.
func NextTargetToBuild(state store.EngineState) *store.ManifestTarget {
	// Don't build anything if there are pending config file changes.
	// We want the Tiltfile to re-run first.
	if len(state.PendingConfigFileChanges) > 0 {
		return nil
	}

	sortedTargets := TopologicalSort(state.Targets())
	sortedTargets = RemoveCurrentlyBuildingTargetsAndDeps(sortedTargets)
	targets := RemoveTargetsWaitingOnDependencies(state, []*store.ManifestTarget(sortedTargets))

	// If any of the manifest targets haven't been built yet, build them now.
	unbuilt := FindTargetsNeedingInitialBuild(targets)

	if len(unbuilt) > 0 {
		ret := NextUnbuiltTargetToBuild(unbuilt)
		return ret
	}

	// Next prioritize builds that crashed and need a rebuilt to have up-to-date code.
	for _, mt := range targets {
		if mt.State.NeedsRebuildFromCrash {
			return mt
		}
	}

	// Next prioritize builds that have been manually triggered.
	if len(state.TriggerQueue) > 0 {
		mn := state.TriggerQueue[0]
		mt, ok := state.ManifestTargets[mn]
		if ok {
			return mt
		}
	}

	return EarliestPendingAutoTriggerTarget(targets)
}

type TopSortedTargets []*store.ManifestTarget

// Sort the resources in topological order,
// so that a target always shows up before any targets that depend on it.
func TopologicalSort(mts []*store.ManifestTarget) TopSortedTargets {
	result := make([]*store.ManifestTarget, 0, len(mts))
	consumed := make(map[model.ManifestName]bool, len(mts))
	consuming := make(map[model.ManifestName]bool, len(mts))
	nameToIndex := make(map[model.ManifestName]int, len(mts))
	for i, mt := range mts {
		nameToIndex[mt.Manifest.Name] = i
	}

	// Declare consume() separately because it's recursive.
	var consume func(i int)
	consume = func(i int) {
		mt := mts[i]
		mn := mt.Manifest.Name
		if consumed[mn] || consuming[mn] {
			return
		}

		consuming[mn] = true

		// Make sure we consume all of a manifest's deps before adding it to the array.
		for _, mn := range mt.Manifest.ResourceDependencies {
			consume(nameToIndex[mn])
		}

		result = append(result, mt)
		consumed[mn] = true
	}

	for i := range mts {
		consume(i)
	}

	return result
}

func NextManifestNameToBuild(state store.EngineState) model.ManifestName {
	mt := NextTargetToBuild(state)
	if mt == nil {
		return ""
	}
	return mt.Manifest.Name
}

func isWaitingOnDependencies(state store.EngineState, mt *store.ManifestTarget) bool {
	// dependencies only block the first build, so if this manifest has ever built, ignore dependencies
	if mt.State.StartedFirstBuild() {
		return false
	}

	for _, mn := range mt.Manifest.ResourceDependencies {
		ms, ok := state.ManifestState(mn)
		if !ok || ms == nil || ms.RuntimeState == nil || !ms.RuntimeState.HasEverBeenReadyOrSucceeded() {
			return true
		}
	}

	return false
}

func RemoveCurrentlyBuildingTargetsAndDeps(mts TopSortedTargets) TopSortedTargets {
	var ret TopSortedTargets
	isBuilding := make(map[model.ManifestName]bool)
	hasBuildingDep := make(map[model.ManifestName]bool)
	for _, mt := range mts {
		mn := mt.Manifest.Name
		if mt.State.IsBuilding() {
			isBuilding[mn] = true
		}

		for _, depName := range mt.Manifest.ResourceDependencies {
			if isBuilding[depName] || hasBuildingDep[depName] {
				hasBuildingDep[mn] = true
			}
		}

		if !(isBuilding[mn] || hasBuildingDep[mn]) {
			ret = append(ret, mt)
		}
	}

	return ret
}

func RemoveTargetsWaitingOnDependencies(state store.EngineState, mts []*store.ManifestTarget) []*store.ManifestTarget {
	var ret []*store.ManifestTarget
	for _, mt := range mts {
		if !isWaitingOnDependencies(state, mt) {
			ret = append(ret, mt)
		}
	}

	return ret
}

// Helper function for ordering targets that have never been built before.
func NextUnbuiltTargetToBuild(unbuilt []*store.ManifestTarget) *store.ManifestTarget {
	// unresourced YAML goes first
	unresourced := FindUnresourcedYAML(unbuilt)
	if unresourced != nil {
		return unresourced
	}

	// Local resources come before all cluster resources (b/c LR's may
	// change things on disk that cluster resources then pull in).
	localTargets := FindLocalTargets(unbuilt)
	if len(localTargets) > 0 {
		return localTargets[0]
	}

	// If this is Kubernetes, unbuilt resources go first.
	// (If this is Docker Compose, we want to trust the ordering
	// that docker-compose put things in.)
	deployOnlyK8sTargets := FindDeployOnlyK8sManifestTargets(unbuilt)
	if len(deployOnlyK8sTargets) > 0 {
		return deployOnlyK8sTargets[0]
	}

	return unbuilt[0]
}

func FindUnresourcedYAML(targets []*store.ManifestTarget) *store.ManifestTarget {
	for _, target := range targets {
		if target.Manifest.ManifestName() == model.UnresourcedYAMLManifestName {
			return target
		}
	}
	return nil
}

func FindDeployOnlyK8sManifestTargets(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if target.Manifest.IsK8s() && len(target.Manifest.ImageTargets) == 0 {
			result = append(result, target)
		}
	}
	return result
}

func FindLocalTargets(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if target.Manifest.IsLocal() {
			result = append(result, target)
		}
	}
	return result
}

// Go through all the manifests, and check:
// 1) all pending file changes, and
// 2) all pending manifest changes
// The earliest one is the one we want.
//
// If no targets are pending, return nil
func EarliestPendingAutoTriggerTarget(targets []*store.ManifestTarget) *store.ManifestTarget {
	var choice *store.ManifestTarget
	earliest := time.Now()

	for _, mt := range targets {
		ok, newTime := mt.State.HasPendingChangesBefore(earliest)
		if ok {
			if !mt.Manifest.TriggerMode.AutoOnChange() {
				// Don't trigger update of a manual manifest just b/c if has
				// pending changes; must come through the TriggerQueue, above.
				continue
			}
			choice = mt
			earliest = newTime
		}
	}

	return choice
}

func FindTargetsNeedingInitialBuild(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if !target.State.StartedFirstBuild() && target.Manifest.TriggerMode.AutoInitial() {
			result = append(result, target)
		}
	}
	return result
}
