package exit

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	tiltruns "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func targetsForResource(mt *store.ManifestTarget, holds buildcontrol.HoldSet) []tiltruns.Target {
	var resources []tiltruns.Target

	buildResource := buildTarget(mt, holds)
	if buildResource != nil {
		resources = append(resources, *buildResource)
	}

	runtimeResource := runtimeTarget(mt, holds)
	if runtimeResource != nil {
		resources = append(resources, *runtimeResource)
	}

	return resources
}

func k8sRuntimeTarget(mt *store.ManifestTarget) *tiltruns.Target {
	krs := mt.State.K8sRuntimeState()
	if mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore && krs.HasEverDeployedSuccessfully && len(krs.Pods) == 0 {
		// HACK: engine assumes anything with an image will create a pod; PodReadinessIgnore is used in these
		// 	instances to avoid getting stuck in pending forever; in reality, there's no "runtime" target being
		// 	monitored by Tilt, so instead of faking it, just omit it (note: only applies AFTER first deploy so
		// 	that we can determine there are no pods, so it will appear in waiting until then, which is actually
		// 	desirable and matches behavior in K8sRuntimeState::RuntimeStatus())
		// 	see https://github.com/tilt-dev/tilt/issues/3619
		return nil
	}

	target := &tiltruns.Target{
		Name:      fmt.Sprintf("%s:runtime", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
	}

	if mt.Manifest.IsK8s() && mt.Manifest.K8sTarget().HasJob() {
		target.Type = tiltruns.TargetTypeJob
	} else {
		target.Type = tiltruns.TargetTypeServer
	}

	// a lot of this logic is duplicated from K8sRuntimeState::RuntimeStatus()
	// but ensures Job containers are handled correctly and adds additional
	// metadata
	pod := krs.MostRecentPod()
	switch pod.Phase {
	case v1.PodRunning:
		target.State.Active = &tiltruns.TargetStateActive{
			StartTime: metav1.NewMicroTime(pod.StartedAt),
			Ready:     mt.Manifest.PodReadinessMode() == model.PodReadinessIgnore || pod.AllContainersReady(),
		}
	case v1.PodSucceeded:
		target.State.Terminated = &tiltruns.TargetStateTerminated{
			StartTime: metav1.NewMicroTime(pod.StartedAt),
		}
	case v1.PodFailed:
		podErr := strings.Join(pod.StatusMessages, "; ")
		if podErr == "" {
			podErr = fmt.Sprintf("Pod %q failed", pod.PodID.String())
		}
		target.State.Terminated = &tiltruns.TargetStateTerminated{
			StartTime: metav1.NewMicroTime(pod.StartedAt),
			Error:     podErr,
		}
	}

	if target.State.Terminated == nil || target.State.Terminated.Error == "" {
		for _, ctr := range pod.AllContainers() {
			if ctr.Status == model.RuntimeStatusError {
				target.State.Terminated = &tiltruns.TargetStateTerminated{
					StartTime: metav1.NewMicroTime(pod.StartedAt),
					Error:     fmt.Sprintf("Pod %s in error state: %s", pod.PodID, pod.Status),
				}
			}
		}
	}

	// default to pending
	if target.State.Active == nil && target.State.Terminated == nil {
		target.State.Waiting = &tiltruns.TargetStateWaiting{
			Reason: pod.Status,
		}
	}

	return target
}

func localServeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *tiltruns.Target {
	if mt.Manifest.LocalTarget().ServeCmd.Empty() {
		// there is no runtime target
		return nil
	}

	target := &tiltruns.Target{
		Name:      fmt.Sprintf("%s:serve", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      tiltruns.TargetTypeServer,
	}

	lrs := mt.State.LocalRuntimeState()
	if runtimeErr := lrs.RuntimeStatusError(); runtimeErr != nil {
		target.State.Terminated = &tiltruns.TargetStateTerminated{
			StartTime:  metav1.NewMicroTime(lrs.StartTime),
			FinishTime: metav1.NewMicroTime(lrs.FinishTime),
			Error:      errToString(runtimeErr),
		}
	} else if lrs.PID != 0 {
		target.State.Active = &tiltruns.TargetStateActive{
			StartTime: metav1.NewMicroTime(lrs.StartTime),
			Ready:     lrs.Ready,
		}
	} else {
		target.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	}

	return target
}

func genericRuntimeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *tiltruns.Target {
	target := &tiltruns.Target{
		Name:      fmt.Sprintf("%s:runtime", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      tiltruns.TargetTypeServer,
	}

	// HACK: RuntimeState is not populated until engine starts builds in some cases; to avoid weird race conditions,
	// 	it defaults to pending assuming the resource isn't _actually_ disabled on startup via auto_init=False
	var runtimeStatus model.RuntimeStatus
	if mt.State.RuntimeState != nil {
		runtimeStatus = mt.State.RuntimeState.RuntimeStatus()
	} else if mt.Manifest.TriggerMode.AutoInitial() {
		runtimeStatus = model.RuntimeStatusPending
	}

	switch runtimeStatus {
	case model.RuntimeStatusPending:
		target.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	case model.RuntimeStatusOK:
		target.State.Active = &tiltruns.TargetStateActive{
			StartTime: metav1.NewMicroTime(mt.State.LastSuccessfulDeployTime),
			// generic resources have no readiness concept so they're just ready by default
			// (this also applies to Docker Compose, since we don't support its health checks)
			Ready: true,
		}
	case model.RuntimeStatusError:
		errMsg := errToString(mt.State.RuntimeState.RuntimeStatusError())
		if errMsg == "" {
			errMsg = "Server target %q failed"
		}
		target.State.Terminated = &tiltruns.TargetStateTerminated{
			Error: errMsg,
		}
	}

	return target
}

func runtimeTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *tiltruns.Target {
	if mt.Manifest.IsK8s() {
		return k8sRuntimeTarget(mt)
	} else if mt.Manifest.IsLocal() {
		return localServeTarget(mt, holds)
	} else {
		return genericRuntimeTarget(mt, holds)
	}
}

func buildTarget(mt *store.ManifestTarget, holds buildcontrol.HoldSet) *tiltruns.Target {
	if mt.Manifest.IsLocal() && mt.Manifest.LocalTarget().UpdateCmd.Empty() {
		return nil
	}

	res := &tiltruns.Target{
		Name:      fmt.Sprintf("%s:build", mt.Manifest.Name.String()),
		Resources: []string{mt.Manifest.Name.String()},
		Type:      tiltruns.TargetTypeJob,
	}

	pendingBuildReason := mt.NextBuildReason()
	if pendingBuildReason != model.BuildReasonNone {
		res.State.Waiting = waitingFromHolds(mt.Manifest.Name, holds)
	} else if !mt.State.CurrentBuild.Empty() {
		res.State.Active = &tiltruns.TargetStateActive{
			StartTime: metav1.NewMicroTime(mt.State.CurrentBuild.StartTime),
		}
	} else if len(mt.State.BuildHistory) != 0 {
		lastBuild := mt.State.LastBuild()
		res.State.Terminated = &tiltruns.TargetStateTerminated{
			StartTime:  metav1.NewMicroTime(lastBuild.StartTime),
			FinishTime: metav1.NewMicroTime(lastBuild.FinishTime),
			Error:      errToString(lastBuild.Error),
		}
	}

	return res
}

func waitingFromHolds(mn model.ManifestName, holds buildcontrol.HoldSet) *tiltruns.TargetStateWaiting {
	// in the API, the reason is not _why_ the target "exists", but rather an explanation for why it's not yet
	// active and is in a pending state (e.g. waitingFromHolds for dependencies)
	waitReason := "unknown"
	if hold, ok := holds[mn]; ok && hold != store.HoldNone {
		waitReason = string(hold)
	}
	return &tiltruns.TargetStateWaiting{
		Reason: waitReason,
	}
}

// tiltfileTarget creates a tiltruns.Target object from the store.ManifestState for the current
// Tiltfile.
//
// This is slightly different from generic resource handling because there is no ManifestTarget in the engine
// for the Tiltfile, just ManifestState, but a lot of the logic is shared/duplicated.
func tiltfileTarget(ms *store.ManifestState) tiltruns.Target {
	tfState := tiltruns.Target{
		Name:      "tiltfile:build",
		Resources: []string{model.TiltfileManifestName.String()},
		Type:      tiltruns.TargetTypeJob,
	}

	// Tiltfile is special in engine state and doesn't have a target, just state, so
	// this logic is largely duplicated from the generic resource build logic
	// N.B. there is currently no concept of a pending Tiltfile build
	if !ms.CurrentBuild.Empty() {
		tfState.State.Active = &tiltruns.TargetStateActive{
			StartTime: metav1.NewMicroTime(ms.CurrentBuild.StartTime),
		}
	} else if len(ms.BuildHistory) != 0 {
		lastBuild := ms.LastBuild()
		tfState.State.Terminated = &tiltruns.TargetStateTerminated{
			StartTime:  metav1.NewMicroTime(lastBuild.StartTime),
			FinishTime: metav1.NewMicroTime(lastBuild.FinishTime),
			Error:      errToString(lastBuild.Error),
		}
	}

	return tfState
}