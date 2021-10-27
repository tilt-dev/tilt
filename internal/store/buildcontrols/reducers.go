package buildcontrols

import (
	"context"
	"fmt"
	"time"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func HandleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	state.StartedBuildCount++

	mn := action.ManifestName
	manifest, ok := state.Manifest(mn)
	if !ok {
		return
	}

	ms, ok := state.ManifestState(mn)
	if !ok {
		return
	}

	bs := model.BuildRecord{
		Edits:     append([]string{}, action.FilesChanged...),
		StartTime: action.StartTime,
		Reason:    action.Reason,
		SpanID:    action.SpanID,
	}
	ms.ConfigFilesThatCausedChange = []string{}
	ms.CurrentBuild = bs

	if ms.IsK8s() {
		krs := ms.K8sRuntimeState()
		for podID := range krs.Pods {
			krs.UpdateStartTime[podID] = action.StartTime
		}
		// remove stale pods
		for podID := range krs.UpdateStartTime {
			if _, ok := krs.Pods[podID]; !ok {
				delete(krs.UpdateStartTime, podID)
			}
		}
	} else if manifest.IsDC() {
		// Attach the SpanID and initialize the runtime state if we haven't yet.
		state, _ := ms.RuntimeState.(dockercompose.State)
		state = state.WithSpanID(runtimelog.SpanIDForDCService(mn))
		ms.RuntimeState = state
	}

	// If this is a full build, we know all the containers will get replaced,
	// so just reset them now.
	//
	// NOTE(nick): Currently, this addresses an issue where the full build deletes
	// the deployment, which then starts killing pods, which we interpret as a
	// crash. A better way to resolve this problem would be to watch for deletions
	// directly. But it's still semantically correct to record that we intended to
	// delete the containers.
	if action.FullBuildTriggered {
		// Reset all the container ids
		ms.LiveUpdatedContainerIDs = container.NewIDSet()
	}

	state.CurrentlyBuilding[mn] = true
	state.RemoveFromTriggerQueue(mn)
}

// When a Manifest build finishes, update the BuildStatus for all applicable
// targets in the engine state.
func handleBuildResults(engineState *store.EngineState,
	mt *store.ManifestTarget, br model.BuildRecord, results store.BuildResultSet) {
	isBuildSuccess := br.Error == nil

	ms := mt.State
	mn := mt.Manifest.Name
	for id, result := range results {
		ms.MutableBuildStatus(id).LastResult = result
	}

	// Remove pending file changes that were consumed by this build.
	for _, status := range ms.BuildStatuses {
		status.ClearPendingChangesBefore(br.StartTime)
	}

	if isBuildSuccess {
		ms.LastSuccessfulDeployTime = br.FinishTime
	}

	// Update build statuses for duplicated image targets in other manifests.
	// This ensures that those images targets aren't redundantly rebuilt.
	for _, currentMT := range engineState.TargetsBesides(mn) {
		// We only want to update image targets for Manifests that are already queued
		// for rebuild and not currently building. This has two benefits:
		//
		// 1) If there's a bug in Tilt's image caching (e.g.,
		//    https://github.com/tilt-dev/tilt/pull/3542), this prevents infinite
		//    builds.
		//
		// 2) If the current manifest build was kicked off by a trigger, we don't
		//    want to queue manifests with the same image.
		if currentMT.NextBuildReason() == model.BuildReasonNone ||
			engineState.IsCurrentlyBuilding(currentMT.Manifest.Name) {
			continue
		}

		currentMS := currentMT.State
		idSet := currentMT.Manifest.TargetIDSet()
		updatedIDSet := make(map[model.TargetID]bool)

		for id, result := range results {
			_, ok := idSet[id]
			if !ok {
				continue
			}

			// We can only reuse image update, not live-updates or other kinds of
			// deploys.
			_, isImageResult := result.(store.ImageBuildResult)
			if !isImageResult {
				continue
			}

			currentStatus := currentMS.MutableBuildStatus(id)
			currentStatus.LastResult = result
			currentStatus.ClearPendingChangesBefore(br.StartTime)
			updatedIDSet[id] = true
		}

		if len(updatedIDSet) == 0 {
			continue
		}

		// Suppose we built manifestA, which contains imageA depending on imageCommon.
		//
		// We also have manifestB, which contains imageB depending on imageCommon.
		//
		// We need to mark imageB as dirty, because it was not built in the manifestA
		// build but its dependency was built.
		//
		// Note that this logic also applies to deploy targets depending on image
		// targets. If we built manifestA, which contains imageX and deploy target
		// k8sA, and manifestB contains imageX and deploy target k8sB, we need to mark
		// target k8sB as dirty so that Tilt actually deploys the changes to imageX.
		rDepsMap := currentMT.Manifest.ReverseDependencyIDs()
		for updatedID := range updatedIDSet {

			// Go through each target depending on an image we just built.
			for _, rDepID := range rDepsMap[updatedID] {

				// If that target was also built, it's up-to-date.
				if updatedIDSet[rDepID] {
					continue
				}

				// Otherwise, we need to mark it for rebuild to pick up the new image.
				currentMS.MutableBuildStatus(rDepID).PendingDependencyChanges[updatedID] = br.StartTime
			}
		}
	}
}

func HandleBuildCompleted(ctx context.Context, engineState *store.EngineState, cb BuildCompleteAction) {
	mn := cb.ManifestName
	defer func() {
		delete(engineState.CurrentlyBuilding, mn)
	}()

	engineState.CompletedBuildCount++

	mt, ok := engineState.ManifestTargets[mn]
	if !ok {
		return
	}

	err := cb.Error
	if err != nil {
		s := fmt.Sprintf("Build Failed: %v", err)

		engineState.LogStore.Append(
			store.NewLogAction(mt.Manifest.Name, cb.SpanID, logger.ErrorLvl, nil, []byte(s)),
			engineState.Secrets)
	}

	ms := mt.State
	bs := ms.CurrentBuild
	bs.Error = err
	bs.FinishTime = cb.FinishTime
	bs.BuildTypes = cb.Result.BuildTypes()
	if bs.SpanID != "" {
		bs.WarningCount = len(engineState.LogStore.Warnings(bs.SpanID))
	}

	ms.AddCompletedBuild(bs)

	ms.CurrentBuild = model.BuildRecord{}
	ms.NeedsRebuildFromCrash = false

	handleBuildResults(engineState, mt, bs, cb.Result)

	if !ms.PendingManifestChange.IsZero() &&
		timecmp.BeforeOrEqual(ms.PendingManifestChange, bs.StartTime) {
		ms.PendingManifestChange = time.Time{}
	}

	if err != nil {
		if IsFatalError(err) {
			engineState.FatalError = err
			return
		}
	} else {
		krs := ms.K8sRuntimeState()
		for podID, pod := range krs.Pods {
			// Reset the baseline, so that we don't show restarts
			// from before any live-updates
			krs.BaselineRestarts[podID] = store.AllPodContainerRestarts(*pod)
		}
	}

	// Track the container ids that have been live-updated whether the
	// build succeeds or fails.
	liveUpdateContainerIDs := cb.Result.LiveUpdatedContainerIDs()
	if len(liveUpdateContainerIDs) == 0 {
		// Assume this was an image build, and reset all the container ids
		ms.LiveUpdatedContainerIDs = container.NewIDSet()
	} else {
		for _, cID := range liveUpdateContainerIDs {
			ms.LiveUpdatedContainerIDs[cID] = true
		}

		krs := ms.K8sRuntimeState()
		bestPod := krs.MostRecentPod()
		if timecmp.AfterOrEqual(bestPod.CreatedAt, bs.StartTime) ||
			timecmp.Equal(krs.UpdateStartTime[k8s.PodID(bestPod.Name)], bs.StartTime) {
			liveupdates.CheckForContainerCrash(engineState, mn.String())
		}
	}

	manifest := mt.Manifest
	if manifest.IsK8s() {
		state := ms.K8sRuntimeState()

		applyFilter := cb.Result.ApplyFilter()
		if applyFilter != nil && len(applyFilter.DeployedRefs) > 0 {
			state.ApplyFilter = applyFilter
		}

		if err == nil {
			state.HasEverDeployedSuccessfully = true
		}

		ms.RuntimeState = state
	}

	if mt.Manifest.IsDC() {
		state, _ := ms.RuntimeState.(dockercompose.State)

		result := cb.Result[mt.Manifest.DockerComposeTarget().ID()]
		dcResult, _ := result.(store.DockerComposeBuildResult)
		cid := dcResult.DockerComposeContainerID
		if cid != "" {
			state = state.WithContainerID(cid)
		}

		cState := dcResult.ContainerState
		if cState != nil {
			state = state.WithContainerState(*cState)
			state = state.WithPorts(dcResult.Ports)

			if docker.HasStarted(*cState) {
				if state.StartTime.IsZero() {
					state = state.WithStartTime(cb.FinishTime)
				}
				if state.LastReadyTime.IsZero() {
					// NB: this will differ from StartTime once we support DC health checks
					state = state.WithLastReadyTime(cb.FinishTime)
				}
			}
		}

		ms.RuntimeState = state
	}

	if mt.Manifest.IsLocal() {
		lrs := ms.LocalRuntimeState()
		if err == nil {
			lt := mt.Manifest.LocalTarget()
			if lt.ReadinessProbe == nil {
				// only update the succeeded time if there's no readiness probe
				lrs.LastReadyOrSucceededTime = time.Now()
			}
			if lt.ServeCmd.Empty() {
				// local resources without a serve command are jobs that run and
				// terminate; so there's no real runtime status
				lrs.Status = v1alpha1.RuntimeStatusNotApplicable
			}
		}
		ms.RuntimeState = lrs
	}
}
