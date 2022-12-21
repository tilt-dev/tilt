package session

import (
	"fmt"
	"sort"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func (r *Reconciler) makeLatestStatus(session *v1alpha1.Session, result *ctrl.Result) v1alpha1.SessionStatus {
	state := r.st.RLockState()
	defer r.st.RUnlockState()

	status := v1alpha1.SessionStatus{
		PID:       session.Status.PID,
		StartTime: session.Status.StartTime,
	}

	// A session only captures services that are created by the main Tiltfile
	// entrypoint. We don't consider any extension Tiltfiles or Manifests created
	// by them.
	ms, ok := state.TiltfileStates[model.MainTiltfileManifestName]
	if ok {
		status.Targets = append(status.Targets, tiltfileTarget(model.MainTiltfileManifestName, ms))
	}

	// determine the reason any resources (and thus all of their targets) are waiting (aka "holds")
	// N.B. we don't actually care about what's "next" to build, but the info comes alongside that
	_, holds := buildcontrol.NextTargetToBuild(state)

	for _, mt := range state.ManifestTargets {
		status.Targets = append(status.Targets, r.targetsForResource(mt, holds, session.Spec.CI, result)...)
	}
	// ensure consistent ordering to avoid unnecessary updates
	sort.SliceStable(status.Targets, func(i, j int) bool {
		return status.Targets[i].Name < status.Targets[j].Name
	})

	r.processExitCondition(session.Spec, &state, &status)
	return status
}

func (r *Reconciler) processExitCondition(spec v1alpha1.SessionSpec, state *store.EngineState, status *v1alpha1.SessionStatus) {
	exitCondition := spec.ExitCondition
	if exitCondition == v1alpha1.ExitConditionManual {
		return
	} else if exitCondition != v1alpha1.ExitConditionCI {
		status.Done = true
		status.Error = fmt.Sprintf("unsupported exit condition: %s", exitCondition)
	}

	allResourcesOK := true
	for _, res := range status.Targets {
		if res.State.Waiting == nil && res.State.Active == nil && res.State.Terminated == nil {
			// if all states are nil, the target has not been requested to run, e.g. auto_init=False
			continue
		}

		isTerminated := res.State.Terminated != nil && res.State.Terminated.Error != ""
		if isTerminated {
			if res.State.Terminated.GraceStatus == v1alpha1.TargetGraceTolerated {
				allResourcesOK = false
				continue
			}

			err := res.State.Terminated.Error
			if res.State.Terminated.GraceStatus == v1alpha1.TargetGraceExceeded {
				err = fmt.Sprintf("exceeded grace period: %v", err)
			}

			status.Done = true
			status.Error = err
			return
		}
		if res.State.Waiting != nil {
			allResourcesOK = false
		} else if res.State.Active != nil && (!res.State.Active.Ready || res.Type == v1alpha1.TargetTypeJob) {
			// jobs must run to completion
			allResourcesOK = false
		}
	}

	// Tiltfile is _always_ a target, so ensure that there's at least one other real target, or it's possible to
	// exit before the targets have actually been initialized
	if allResourcesOK && len(status.Targets) > 1 {
		status.Done = true
	}
}

// errToString returns a stringified version of an error or an empty string if the error is nil.
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
