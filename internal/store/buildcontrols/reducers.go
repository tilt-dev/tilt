package buildcontrols

import (
	"context"
	"fmt"
	"time"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockercomposeservices"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const BuildControlSource = "buildcontrol"

func HandleBuildStarted(ctx context.Context, state *store.EngineState, action BuildStartedAction) {
	if action.Source == BuildControlSource {
		state.BuildControllerStartCount++
	}

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
	ms.CurrentBuilds[action.Source] = bs

	if ms.IsK8s() {
		krs := ms.K8sRuntimeState()
		podIDSet := map[k8s.PodID]bool{}
		for _, pod := range krs.GetPods() {
			podIDSet[k8s.PodID(pod.Name)] = true
			krs.UpdateStartTime[k8s.PodID(pod.Name)] = action.StartTime
		}
		// remove stale pods
		for podID := range krs.UpdateStartTime {
			if !podIDSet[podID] {
				delete(krs.UpdateStartTime, podID)
			}
		}
	} else if manifest.IsDC() {
		// Attach the SpanID and initialize the runtime state if we haven't yet.
		state, _ := ms.RuntimeState.(dockercompose.State)
		state = state.WithSpanID(dockercomposeservices.SpanIDForDCService(mn))
		ms.RuntimeState = state
	}

	state.RemoveFromTriggerQueue(mn)
	state.CurrentBuildSet[mn] = true
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
		status.ConsumeChangesBefore(br.StartTime)
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
			engineState.IsBuilding(currentMT.Manifest.Name) {
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
			currentStatus.ConsumeChangesBefore(br.StartTime)
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
				currentMS.MutableBuildStatus(rDepID).DependencyChanges[updatedID] = br.StartTime
			}
		}
	}
}

func HandleBuildCompleted(ctx context.Context, engineState *store.EngineState, cb BuildCompleteAction) {
	mn := cb.ManifestName
	defer func() {
		if !engineState.IsBuilding(mn) {
			delete(engineState.CurrentBuildSet, mn)
		}
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
	bs := ms.CurrentBuilds[cb.Source]
	bs.Error = err
	bs.FinishTime = cb.FinishTime
	bs.BuildTypes = cb.Result.BuildTypes()
	if bs.SpanID != "" {
		bs.WarningCount = len(engineState.LogStore.Warnings(bs.SpanID))
	}

	ms.AddCompletedBuild(bs)

	delete(ms.CurrentBuilds, cb.Source)

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
		cid := dcResult.Status.ContainerID
		if cid != "" {
			state = state.WithContainerID(container.ID(cid))
		}

		cState := dcResult.Status.ContainerState
		if cState != nil {
			state = state.WithContainerState(*cState)
			state = state.WithPorts(dcResult.Status.PortBindings)
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
