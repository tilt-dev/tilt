package buildcontrol

import (
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Algorithm to choose a manifest to build next.
//
// The HoldSet is used in the UI to display why a resource is waiting.
func NextTargetToBuild(state store.EngineState) (*store.ManifestTarget, HoldSet) {
	holds := HoldSet{}

	// Only grab the targets that need any builds at all,
	// so that we don't put holds on builds that aren't even eligible.
	targets := FindTargetsNeedingAnyBuild(state)

	// Don't build anything if there are pending config file changes.
	// We want the Tiltfile to re-run first.
	for _, ms := range state.GetTiltfileStates() {
		tiltfileHasPendingChanges, _ := ms.HasPendingChanges()
		if tiltfileHasPendingChanges {
			holds.Fill(targets, store.Hold{
				Reason: store.HoldReasonTiltfileReload,
				HoldOn: []model.TargetID{ms.TargetID()},
			})
			return nil, holds
		}
	}

	// We do not know whether targets are enabled or disabled until their configmaps + uiresources are synced
	// and reconciled. This happens very quickly after the first Tiltfile execution.
	// If any targets have an unknown EnableStatus, then we don't have enough information to schedule builds:
	// - If we treat an unknown as disabled but it is actually enabled, then we break our heuristic prioritization
	//   (e.g., we might schedule k8s resources before local resources).
	// - If we treat an unknown as enabled but it is actually disabled, then we start logging + side-effecting
	//   a build that might immediately be canceled.
	if pending := TargetsWithPendingEnableStatus(targets); len(pending) > 0 {
		holds.Fill(targets, store.Hold{
			Reason: store.HoldReasonTiltfileReload,
			HoldOn: pending,
		})
		return nil, holds
	}

	// If we're already building an unparallelizable local target, bail immediately.
	if mn, _, building := IsBuildingUnparallelizableLocalTarget(state); building {
		holds.Fill(targets, store.Hold{
			Reason: store.HoldReasonWaitingForUnparallelizableTarget,
			HoldOn: []model.TargetID{mn.TargetID()},
		})
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
	// TODO(nick): Long-term, we should try to infer dependencies between Kubernetes
	// resources. A general library might make sense.
	if IsBuildingUncategorizedYAML(state) {
		HoldK8sTargets(targets, holds)
	}

	HoldTargetsWithBuildingComponents(state, targets, holds)
	HoldTargetsWaitingOnDependencies(state, targets, holds)
	HoldTargetsWaitingOnCluster(state, targets, holds)

	// If any of the manifest targets haven't been built yet, build them now.
	targets = holds.RemoveIneligibleTargets(targets)
	unbuilt := FindTargetsNeedingInitialBuild(targets)

	if len(unbuilt) > 0 {
		return NextUnbuiltTargetToBuild(unbuilt), holds
	}

	// Check to see if any targets are currently being successfully reconciled,
	// and so full rebuilt should be held back. This takes manual triggers into account.
	HoldLiveUpdateTargetsHandledByReconciler(state, targets, holds)

	// Next prioritize builds that have been manually triggered.
	for _, mn := range state.TriggerQueue {
		mt, ok := state.ManifestTargets[mn]
		if ok && holds.IsEligible(mt) {
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

func waitingOnDependencies(state store.EngineState, mt *store.ManifestTarget) []model.TargetID {
	// dependencies only block the first build, so if this manifest has ever built, ignore dependencies
	if mt.State.StartedFirstBuild() {
		return nil
	}

	var waitingOn []model.TargetID
	for _, mn := range mt.Manifest.ResourceDependencies {
		ms, ok := state.ManifestState(mn)
		if !ok || ms == nil || ms.RuntimeState == nil || !ms.RuntimeState.HasEverBeenReadyOrSucceeded() {
			waitingOn = append(waitingOn, mn.TargetID())
		}
	}

	return waitingOn
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
func canReuseImageTargetHeuristic(spec model.TargetSpec, status *store.BuildStatus) bool {
	id := spec.ID()
	if id.Type != model.TargetTypeImage {
		return false
	}

	// NOTE(nick): A more accurate check might see if the pending file changes
	// are potentially live-updatable, but this is OK for the case of a base image.
	if status.HasPendingFileChanges() || status.HasPendingDependencyChanges() {
		return false
	}

	result := status.LastResult
	if result == nil {
		return false
	}

	switch result.(type) {
	case store.ImageBuildResult:
		return true
	}
	return false
}

func HoldTargetsWithBuildingComponents(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	building := make(map[model.TargetID]bool)

	for _, mt := range state.Targets() {
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

	hasBuildingComponent := func(mt *store.ManifestTarget) ([]model.TargetID, bool) {
		var targetIDs []model.TargetID
		var shouldHold bool

		m := mt.Manifest
		if building[m.ID()] {
			// mark as holding but don't add self as a dependency
			shouldHold = true
		}

		for _, spec := range m.TargetSpecs() {
			if canReuseImageTargetHeuristic(spec, mt.State.BuildStatus(spec.ID())) {
				continue
			}

			if building[spec.ID()] {
				targetIDs = append(targetIDs, spec.ID())
				shouldHold = true
			}
		}
		return targetIDs, shouldHold
	}

	for _, mt := range mts {
		if waitingOn, shouldHold := hasBuildingComponent(mt); shouldHold {
			holds.AddHold(mt, store.Hold{
				Reason: store.HoldReasonBuildingComponent,
				HoldOn: waitingOn,
			})
		}
	}
}

func targetsByCluster(mts []*store.ManifestTarget) map[string][]*store.ManifestTarget {
	clusters := make(map[string][]*store.ManifestTarget)
	for _, mt := range mts {
		clusterName := mt.Manifest.ClusterName()
		if clusterName == "" {
			continue
		}

		targets, ok := clusters[clusterName]
		if !ok {
			targets = []*store.ManifestTarget{}
		}
		clusters[clusterName] = append(targets, mt)
	}
	return clusters
}

// We use the cluster to detect what architecture we're building for.
// Until the cluster connection has been established, we block any
// image builds.
func HoldTargetsWaitingOnCluster(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	for clusterName, targets := range targetsByCluster(mts) {
		cluster, ok := state.Clusters[clusterName]
		isClusterOK := ok && cluster.Status.Error == "" && cluster.Status.Arch != ""
		if isClusterOK {
			continue
		}

		gvk := v1alpha1.SchemeGroupVersion.WithKind("Cluster")
		for _, mt := range targets {
			holds.AddHold(mt, store.Hold{
				Reason: store.HoldReasonCluster,
				OnRefs: []v1alpha1.UIResourceStateWaitingOnRef{{
					Group:      gvk.Group,
					APIVersion: gvk.Version,
					Kind:       gvk.Kind,
					Name:       clusterName,
				}},
			})
		}
	}
}

func HoldTargetsWaitingOnDependencies(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	for _, mt := range mts {
		if waitingOn := waitingOnDependencies(state, mt); len(waitingOn) != 0 {
			holds.AddHold(mt, store.Hold{
				Reason: store.HoldReasonWaitingForDep,
				HoldOn: waitingOn,
			})
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
			holds[target.Manifest.Name] = store.Hold{Reason: store.HoldReasonIsUnparallelizableTarget}
		}
	}
}

func HoldK8sTargets(targets []*store.ManifestTarget, holds HoldSet) {
	for _, target := range targets {
		if target.Manifest.IsK8s() {
			holds.AddHold(target, store.Hold{
				Reason: store.HoldReasonWaitingForUncategorized,
				HoldOn: []model.TargetID{model.UnresourcedYAMLManifestName.TargetID()},
			})
		}
	}
}

func TargetsWithPendingEnableStatus(targets []*store.ManifestTarget) []model.TargetID {
	var result []model.TargetID
	for _, target := range targets {
		if target.State.DisableState == v1alpha1.DisableStatePending {
			result = append(result, target.Spec().ID())
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

func IsBuildingUnparallelizableLocalTarget(state store.EngineState) (model.ManifestName, model.TargetName, bool) {
	mts := state.Targets()
	for _, mt := range mts {
		if mt.State.IsBuilding() && mt.Manifest.IsLocal() &&
			!mt.Manifest.LocalTarget().AllowParallel {
			return mt.Manifest.Name, mt.Manifest.LocalTarget().Name, true
		}
	}
	return "", "", false
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

// Grab all the targets that are build-eligible from
// the engine state.
//
// We apply this filter first, then layer on individual build decisions about
// what to build next. This MUST be the union of all checks in all downstream
// build decisions in NextTargetToBuild.
func FindTargetsNeedingAnyBuild(state store.EngineState) []*store.ManifestTarget {
	queue := make(map[model.ManifestName]bool, len(state.TriggerQueue))
	for _, mn := range state.TriggerQueue {
		queue[mn] = true
	}

	result := []*store.ManifestTarget{}
	for _, target := range state.Targets() {
		// Skip disabled targets.
		if target.State.DisableState == v1alpha1.DisableStateDisabled {
			continue
		}

		if !target.State.StartedFirstBuild() && target.Manifest.TriggerMode.AutoInitial() {
			result = append(result, target)
			continue
		}

		if queue[target.Manifest.Name] {
			result = append(result, target)
			continue
		}

		hasPendingChanges, _ := target.State.HasPendingChanges()
		if hasPendingChanges && target.Manifest.TriggerMode.AutoOnChange() {
			result = append(result, target)
			continue
		}
	}

	return result
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
			holds.AddHold(mt, store.Hold{Reason: store.HoldReasonWaitingForDeploy})
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
		if !status.HasPendingFileChanges() {
			continue
		}

		// We have an image target with changes!
		// First, make sure that all the changes match a sync.
		files := status.PendingFileChangesList()
		iTarget := mt.Manifest.ImageTargetWithID(id)
		luSpec := iTarget.LiveUpdateSpec
		_, pathsMatchingNoSync, err := build.FilesToPathMappings(files, liveupdate.SyncSteps(luSpec))
		if err != nil || len(pathsMatchingNoSync) > 0 {
			return false
		}

		// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
		anyMatch, _, err := liveupdate.FallBackOnFiles(luSpec).AnyMatch(files)
		if err != nil || anyMatch {
			return false
		}

		// All changes match a sync!
		//
		// We only care about targets where there are 0 running containers for the current build.
		// This is the mechanism that live update uses to determine if the container to live-update
		// is still pending.
		if mt.Manifest.IsK8s() && iTarget.LiveUpdateSpec.Selector.Kubernetes != nil {
			kResource := state.KubernetesResources[mt.Manifest.Name.String()]
			if kResource == nil {
				return true // Wait for the k8s resource to appear.
			}

			cInfos, err := liveupdates.RunningContainersForOnePod(
				iTarget.LiveUpdateSpec.Selector.Kubernetes,
				kResource,
				state.ImageMaps[iTarget.ImageMapName()],
			)
			if err != nil {
				return false
			}

			if len(cInfos) != 0 {
				return false
			}

			// If the container in this pod is in a crash loop, then don't hold back
			// updates until the deploy finishes -- this is a pretty good signal
			// that it might not become healthy.
			pod := k8sconv.MostRecentPod(kResource.FilteredPods)
			for _, c := range pod.Containers {
				if c.Restarts > 0 {
					return false
				}
			}

			// If the pod is in a finished state, then the containers
			// may never re-enter Running.
			if pod.Phase == string(v1.PodSucceeded) || pod.Phase == string(v1.PodFailed) {
				return false
			}

		} else if mt.Manifest.IsDC() {
			dcs := state.DockerComposeServices[mt.Manifest.Name.String()]
			cInfos := liveupdates.RunningContainersForDC(dcs)
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

// Hold back live update targets that are being successfully
// handled by a reconciler.
func HoldLiveUpdateTargetsHandledByReconciler(state store.EngineState, mts []*store.ManifestTarget, holds HoldSet) {
	for _, mt := range mts {
		// Most types of build reasons trigger a full rebuild. The two exceptions are:
		// - File-change only
		// - Live-update eligible manual triggers
		reason := mt.NextBuildReason()
		isLiveUpdateEligible := reason == model.BuildReasonFlagChangedFiles
		if reason.HasTrigger() {
			isLiveUpdateEligible = IsLiveUpdateEligibleTrigger(mt.Manifest, reason)
		}

		if !isLiveUpdateEligible {
			continue
		}

		// Changes to the deploy target can't be live-updated.
		if mt.Manifest.DeployTarget != nil {
			bs, hasBuildStatus := mt.State.BuildStatuses[mt.Manifest.DeployTarget.ID()]
			hasPendingChanges := hasBuildStatus && bs.HasPendingFileChanges()
			if hasPendingChanges {
				continue
			}
		}

		allHandledByLiveUpdate := true
		iTargets := mt.Manifest.ImageTargets
		for _, iTarget := range iTargets {
			bs, hasBuildStatus := mt.State.BuildStatuses[iTarget.ID()]
			hasPendingChanges := hasBuildStatus && bs.HasPendingFileChanges()
			if !hasPendingChanges {
				continue
			}

			handlers := findLiveUpdateHandlers(iTarget, mt, &state)
			if len(handlers) == 0 {
				allHandledByLiveUpdate = false
			}

			for _, lu := range handlers {
				isFailing := lu.Status.Failed != nil
				if isFailing {
					allHandledByLiveUpdate = false
				}
			}

			if !allHandledByLiveUpdate {
				break
			}
		}

		if allHandledByLiveUpdate {
			holds.AddHold(mt, store.Hold{Reason: store.HoldReasonReconciling})
		}
	}
}

// Find all the live update objects responsible for syncing this image.
//
// Base image live updates are modeled with a LiveUpdate object attached to
// each deploy image.
//
// The LiveUpdate watches:
// - The Deploy image's container
// - The Base image's filewatch
//
// The Tiltfile assembler will guarantee that there will be one LiveUpdate
// object for each deployed image, and they will all sync in the same way.
func findLiveUpdateHandlers(changedImage model.ImageTarget, mt *store.ManifestTarget, state *store.EngineState) []*v1alpha1.LiveUpdate {
	result := []*v1alpha1.LiveUpdate{}

	for _, candidate := range mt.Manifest.ImageTargets {
		isHandledByReconciler := !liveupdate.IsEmptySpec(candidate.LiveUpdateSpec) &&
			candidate.LiveUpdateReconciler
		if !isHandledByReconciler {
			continue
		}

		lu := state.LiveUpdates[candidate.LiveUpdateName]
		if lu == nil {
			continue
		}

		isHandled := false
		for _, source := range lu.Spec.Sources {
			// Relies on the assumption that image targets create filewatches
			// with the same name.
			if source.FileWatch == changedImage.ID().String() {
				isHandled = true
				break
			}
		}

		if isHandled {
			result = append(result, lu)
		}
	}

	return result
}

// In automatic trigger mode:
// - Clicking the trigger button always triggers a full rebuild.
//
// In manual trigger mode:
// - If there are no pending changes, clicking the trigger button triggers a full rebuild.
// - If there are only pending changes, clicking the trigger button triggers a live-update.
func IsLiveUpdateEligibleTrigger(manifest model.Manifest, reason model.BuildReason) bool {
	return reason.HasTrigger() &&
		reason.WithoutTriggers() == model.BuildReasonFlagChangedFiles &&
		!manifest.TriggerMode.AutoOnChange()
}
