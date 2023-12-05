package session

import (
	"fmt"
	"sort"
	"strings"

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

	// If there's a global timeout, schedule a requeue.
	ci := session.Spec.CI
	if ci != nil && ci.Timeout != nil && ci.Timeout.Duration > 0 {
		timeout := ci.Timeout.Duration
		requeueAfter := timeout - r.clock.Since(session.Status.StartTime.Time)
		if result.RequeueAfter == 0 || result.RequeueAfter > requeueAfter {
			result.RequeueAfter = requeueAfter
		}
	}

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

	var waiting []string
	var notReady []string
	var retrying []string

	allResourcesOK := func() bool {
		return len(waiting)+len(notReady)+len(retrying) == 0
	}

	for _, res := range status.Targets {
		if res.State.Waiting == nil && res.State.Active == nil && res.State.Terminated == nil {
			// if all states are nil, the target has not been requested to run, e.g. auto_init=False
			continue
		}

		isTerminated := res.State.Terminated != nil && res.State.Terminated.Error != ""
		if isTerminated {
			if res.State.Terminated.GraceStatus == v1alpha1.TargetGraceTolerated {
				retrying = append(retrying, res.Name)
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
			waiting = append(waiting, fmt.Sprintf("%v %v", res.Name, res.State.Waiting.WaitReason))
		} else if res.State.Active != nil && !res.State.Active.Ready {
			// jobs must run to completion
			notReady = append(notReady, res.Name)
		}
	}

	// Tiltfile is _always_ a target, so ensure that there's at least one other real target, or it's possible to
	// exit before the targets have actually been initialized
	if allResourcesOK() && len(status.Targets) > 1 {
		status.Done = true
	}

	summary := func() string {
		buf := new(strings.Builder)
		for _, category := range []struct {
			name  string
			items []string
		}{
			{name: "waiting", items: waiting},
			{name: "not ready", items: notReady},
			{name: "retrying", items: retrying},
		} {
			if num := len(category.items); num > 0 {
				if buf.Len() > 0 {
					buf.WriteString(", ")
				}
				fmt.Fprintf(buf, "%d resources %v (%v)",
					num, category.name, strings.Join(category.items, ","))
			}
		}
		return buf.String()
	}

	// Enforce a global timeout.
	ci := spec.CI
	if status.Error == "" && ci != nil && ci.Timeout != nil && ci.Timeout.Duration > 0 &&
		r.clock.Since(status.StartTime.Time) > ci.Timeout.Duration {
		status.Done = true
		status.Error = fmt.Sprintf("Timeout after %s: %v", ci.Timeout.Duration, summary())
	}
}

// errToString returns a stringified version of an error or an empty string if the error is nil.
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
