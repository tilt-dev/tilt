package buildcontrol

import (
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
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

	// Local targets aren't parallelizable with any other kind of target.
	// So if we're already building a local target, bail immediately.
	if IsBuildingLocalTarget(state) {
		return nil
	}

	targets := state.Targets()
	if IsBuildingAnything(state) {
		// Local targets aren't parallelizable with any other kind of target.
		// So if we're already building any targets, remove all the local ones.
		targets = RemoveLocalTargets(targets)
	}

	targets = RemoveTargetsWithBuildingComponents(targets)
	targets = RemoveTargetsWaitingOnDependencies(state, targets)

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

func RemoveTargetsWithBuildingComponents(mts []*store.ManifestTarget) []*store.ManifestTarget {
	building := make(map[model.TargetID]bool)

	for _, mt := range mts {
		if mt.State.IsBuilding() {
			building[mt.Manifest.ID()] = true

			// TODO(nick): This logic isn't quite right.  A manifest can re-use image
			// results from another manifest's build.
			for _, spec := range mt.Manifest.TargetSpecs() {
				building[spec.ID()] = true
			}
		}
	}

	isBuilding := func(m model.Manifest) bool {
		if building[m.ID()] {
			return true
		}

		for _, spec := range m.TargetSpecs() {
			if building[spec.ID()] {
				return true
			}
		}
		return false
	}

	var ret []*store.ManifestTarget
	for _, mt := range mts {
		if !isBuilding(mt.Manifest) {
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
	// Local resources come before all cluster resources, because they
	// can't be parallelized. (LR's may change things on disk that cluster
	// resources then pull in).
	localTargets := FindLocalTargets(unbuilt)
	if len(localTargets) > 0 {
		return localTargets[0]
	}

	// unresourced YAML goes next
	unresourced := FindUnresourcedYAML(unbuilt)
	if unresourced != nil {
		return unresourced
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

func RemoveLocalTargets(targets []*store.ManifestTarget) []*store.ManifestTarget {
	result := []*store.ManifestTarget{}
	for _, target := range targets {
		if !target.Manifest.IsLocal() {
			result = append(result, target)
		}
	}
	return result
}

func IsBuildingAnything(state store.EngineState) bool {
	mts := state.Targets()
	for _, mt := range mts {
		if mt.State.IsBuilding() {
			return true
		}
	}
	return false
}

func IsBuildingLocalTarget(state store.EngineState) bool {
	mts := state.Targets()
	for _, mt := range mts {
		if mt.State.IsBuilding() && mt.Manifest.IsLocal() {
			return true
		}
	}
	return false
}

// Go through all the manifests, and check:
// 1) all pending file changes
// 2) all pending dependency changes (where an image has been rebuilt by another manifest), and
// 3) all pending manifest changes
// The earliest one is the one we want.
//
// If no targets are pending, return nil
func EarliestPendingAutoTriggerTarget(targets []*store.ManifestTarget) *store.ManifestTarget {
	var choice *store.ManifestTarget
	earliest := time.Now()

	for _, mt := range targets {
		ok, newTime := mt.State.HasPendingChangesBeforeOrEqual(earliest)
		if ok {
			if !mt.Manifest.TriggerMode.AutoOnChange() {
				// Don't trigger update of a manual manifest just b/c if has
				// pending changes; must come through the TriggerQueue, above.
				continue
			}
			if choice != nil && newTime.Equal(earliest) {
				// If two choices are equal, use the first one in target order.
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
