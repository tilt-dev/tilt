package buildcontrol

import (
	"time"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// NOTE(maia): we eventually want to move the BuildController into its own package
// (as we do with all subscribers), but for now, just move the underlying functions
// so they can be used from elsewhere.

// Algorithm to choose a manifest to build next.
func NextTargetToBuild(state store.EngineState) (*store.ManifestTarget, HoldSet) {
	holds := HoldSet{}
	targets := state.Targets()

	// Don't build anything if there are pending config file changes.
	// We want the Tiltfile to re-run first.
	tiltfileHasPendingChanges, _ := state.TiltfileState.HasPendingChanges()
	if tiltfileHasPendingChanges {
		holds.Fill(targets, store.HoldTiltfileReload)
		return nil, holds
	}

	// If we're already building an unparallelizable local target, bail immediately.
	if IsBuildingUnparallelizableLocalTarget(state) {
		holds.Fill(targets, store.HoldWaitingForUnparallelizableTarget)
		return nil, holds
	}

	if IsBuildingAnything(state) {
		// If we're building a target already, remove anything that's not parallelizable
		// with what's currently building.
		HoldUnparallelizableLocalTargets(targets, holds)
	}

	// Uncategorized YAML might contain namespaces or volumes that
	// we don't want to parallelize.
	//
	// TODO(nick): Long-term, we should try to infer dependencies between Kuberentes
	// resources. A general library might make sense.
	if IsBuildingUncategorizedYAML(state) {
		HoldK8sTargets(targets, holds)
	}

	HoldTargetsWithBuildingComponents(targets, holds)
	HoldTargetsWaitingOnDependencies(state, targets, holds)

	// If any of the manifest targets haven't been built yet, build them now.
	targets = holds.RemoveIneligibleTargets(targets)
	unbuilt := FindTargetsNeedingInitialBuild(targets)

	if len(unbuilt) > 0 {
		return NextUnbuiltTargetToBuild(unbuilt), holds
	}

	// Next prioritize builds that crashed and need a rebuilt to have up-to-date code.
	for _, mt := range targets {
		if mt.State.NeedsRebuildFromCrash {
			return mt, holds
		}
	}

	// Next prioritize builds that have been manually triggered.
	if len(state.TriggerQueue) > 0 {
		mn := state.TriggerQueue[0]
		mt, ok := state.ManifestTargets[mn]
		if ok {
			return mt, holds
		}
	}

	// Check to see if any targets
	//
	// 1) Have live updates
	// 2) All the pending file changes are completely captured by the live updates
	// 3) The runtime is in a pending state
	//
	// This will ensure that a file change doesn't accidentally overwrite
	// a pending pod.
	//
	// https://github.com/tilt-dev/tilt/issues/3759
	HoldLiveUpdateTargetsWaitingOnDeploy(state, targets, holds)
	targets = holds.RemoveIneligibleTargets(targets)

	return EarliestPendingAutoTriggerTarget(targets), holds
}

func NextManifestNameToBuild(state store.EngineState) model.ManifestName {
	mt, _ := NextTargetToBuild(state)
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

// Check to see if this is an ImageTarget where the built image
// can be potentially reused.
//
// Note that this is a quick heuristic check for making parallelization decisions.
//
// The "correct" decision about whether an image can be re-used is more complex
// and expensive, and includes:
//
// 1) Checks of dependent images
// 2) Live-update sync checks
// 3) Checks that the image still exists on the image store
//
// But in this particular context, we can cheat a bit.
func canReuseImageTargetHeuristic(spec model.TargetSpec, status store.BuildStatus) bool {
	id := spec.ID()
	if id.Type != model.TargetTypeImage {
		return false
	}

	// NOTE(nick): A more accurate check might see if the pending file changes
	// are potentially live-updatable, but this is OK for the case of a base image.
	if len(status.PendingFileChanges) > 0 || len(status.PendingDependencyChanges) > 0 {
		return false
	}

	result := status.LastResult
	if result == nil {
		return false
	}

	switch result.(type) {
	case store.ImageBuildResult, store.LiveUpdateBuildResult:
		return true
	}
	return false
}

func HoldTargetsWithBuildingComponents(mts []*store.ManifestTarget, holds HoldSet) {
	building := make(map[model.TargetID]bool)

	for _, mt := range mts {
		if mt.State.IsBuilding() {
			building[mt.Manifest.ID()] = true

			for _, spec := range mt.Manifest.TargetSpecs() {
				if canReuseImageTargetHeuristic(spec, mt.State.BuildStatus(spec.ID())) {
					continue
				}

				building[spec.ID()] = true
			}
		}
	}

	hasBuildingComponent := func(mt *store.ManifestTarget) bool {
		m := mt.Manifest
		if building[m.ID()] {
			return true
		}

		for _, spec := range m.TargetSpecs() {
			if canReuseImageTargetHeuristic(spec, mt.State.BuildStatus(spec.ID())) {
				continue
			}

			if building[spec.ID()] {
				return true
			}
		}
		return false
	}

	for _, mt := range mts {
		if hasBuildingComponent(mt) {
			holds.AddHold(mt, store.HoldBuildingComponent)
		}
	}
}

func HoldTargetsWaitingOnDependencies(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	for _, mt := range mts {
		if isWaitingOnDependencies(state, mt) {
			holds.AddHold(mt, store.HoldWaitingForDep)
		}
	}
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

func HoldUnparallelizableLocalTargets(targets []*store.ManifestTarget, holds map[model.ManifestName]store.Hold) {
	for _, target := range targets {
		if target.Manifest.IsLocal() && !target.Manifest.LocalTarget().AllowParallel {
			holds[target.Manifest.Name] = store.HoldIsUnparallelizableTarget
		}
	}
}

func HoldK8sTargets(targets []*store.ManifestTarget, holds HoldSet) {
	for _, target := range targets {
		if target.Manifest.IsK8s() {
			holds.AddHold(target, store.HoldWaitingForUncategorized)
		}
	}
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

func IsBuildingUnparallelizableLocalTarget(state store.EngineState) bool {
	mts := state.Targets()
	for _, mt := range mts {
		if mt.State.IsBuilding() && mt.Manifest.IsLocal() &&
			!mt.Manifest.LocalTarget().AllowParallel {
			return true
		}
	}
	return false
}

func IsBuildingUncategorizedYAML(state store.EngineState) bool {
	mts := state.Targets()
	for _, mt := range mts {
		if mt.State.IsBuilding() && mt.Manifest.Name == model.UnresourcedYAMLManifestName {
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

func HoldLiveUpdateTargetsWaitingOnDeploy(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	for _, mt := range mts {
		if IsLiveUpdateTargetWaitingOnDeploy(state, mt) {
			holds.AddHold(mt, store.HoldWaitingForDeploy)
		}
	}
}

func IsLiveUpdateTargetWaitingOnDeploy(state store.EngineState, mt *store.ManifestTarget) bool {
	// We only care about targets where file changes are the ONLY build reason.
	if mt.NextBuildReason() != model.BuildReasonFlagChangedFiles {
		return false
	}

	// Make sure the last build succeeded.
	if mt.State.LastBuild().Empty() || mt.State.LastBuild().Error != nil {
		return false
	}

	// Never hold back a deploy in an error state.
	if mt.State.RuntimeState.RuntimeStatus() == v1alpha1.RuntimeStatusError {
		return false
	}

	// Go through all the files, and make sure they're live-update-able.
	for id, status := range mt.State.BuildStatuses {
		if len(status.PendingFileChanges) == 0 {
			continue
		}

		// We have an image target with changes!
		// First, make sure that all the changes match a sync.
		files := make([]string, 0, len(status.PendingFileChanges))
		for f := range status.PendingFileChanges {
			files = append(files, f)
		}

		iTarget := mt.Manifest.ImageTargetWithID(id)
		luInfo := iTarget.LiveUpdateInfo()
		_, pathsMatchingNoSync, err := build.FilesToPathMappings(files, luInfo.SyncSteps())
		if err != nil || len(pathsMatchingNoSync) > 0 {
			return false
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, _, err := luInfo.FallBackOnFiles().AnyMatch(files)
		if err != nil || anyMatch {
			return false
		}

		// All changes match a sync!
		//
		// We only care about targets where there are 0 running containers for the current build.
		// This is the mechanism that live update uses to determine if the container to live-update
		// is still pending.
		if mt.Manifest.IsK8s() {
			cInfos, err := store.RunningContainersForTargetForOnePod(iTarget, mt.State.K8sRuntimeState())
			if err != nil {
				return false
			}

			if len(cInfos) != 0 {
				return false
			}

			// If the container in this pod is in a crash loop, then don't hold back
			// updates until the deploy finishes -- this is a pretty good signal
			// that it might not become healthy.
			pod := mt.State.K8sRuntimeState().MostRecentPod()
			for _, c := range pod.Containers {
				if c.Restarts > 0 {
					return false
				}
			}

		} else if mt.Manifest.IsDC() {
			cInfos := store.RunningContainersForDC(mt.State.DCRuntimeState())
			if len(cInfos) != 0 {
				return false
			}
		} else {
			return false
		}
	}

	// If we've gotten this far, that means we should wait until this deploy
	// finishes before processing these file changes.
	return true
}
