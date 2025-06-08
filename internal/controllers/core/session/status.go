package session

import (
	"fmt"
	"sort"
	"strings"
	"time"

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

	r.processExitCondition(session.Spec, &state, &status, result)

	return status
}

func (r *Reconciler) processExitCondition(spec v1alpha1.SessionSpec, state *store.EngineState, status *v1alpha1.SessionStatus, result *ctrl.Result) {
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

	r.enforceReadinessTimeout(spec, status, result)

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

	r.enforceGlobalTimeout(spec, status, summary, result)
}

func (r *Reconciler) enforceGlobalTimeout(spec v1alpha1.SessionSpec, status *v1alpha1.SessionStatus, summaryFn func() string, result *ctrl.Result) {
	if status.Done {
		return
	}

	ci := spec.CI
	if ci == nil || ci.Timeout == nil || ci.Timeout.Duration <= 0 {
		return
	}

	elapsed := r.clock.Since(status.StartTime.Time)
	if elapsed > ci.Timeout.Duration {
		status.Done = true
		status.Error = fmt.Sprintf("Timeout after %s: %v", ci.Timeout.Duration, summaryFn())
	} else {
		remaining := ci.Timeout.Duration - elapsed
		if result.RequeueAfter == 0 || result.RequeueAfter > remaining {
			result.RequeueAfter = remaining
		}
	}
}

func (r *Reconciler) enforceReadinessTimeout(spec v1alpha1.SessionSpec, status *v1alpha1.SessionStatus, result *ctrl.Result) {
	if status.Done {
		return
	}
	if spec.CI == nil || spec.CI.ReadinessTimeout == nil || spec.CI.ReadinessTimeout.Duration <= 0 {
		return
	}
	readinessTimeout := spec.CI.ReadinessTimeout.Duration
	minRemaining := readinessTimeout
	for _, target := range status.Targets {
		if target.State.Active == nil || target.State.Active.Ready {
			continue
		}
		var refTime time.Time
		if !target.State.Active.LastReadyTime.IsZero() {
			refTime = target.State.Active.LastReadyTime.Time
		} else if !target.State.Active.StartTime.IsZero() {
			refTime = target.State.Active.StartTime.Time
		}
		if refTime.IsZero() {
			continue
		}
		elapsed := r.clock.Since(refTime)
		if elapsed > readinessTimeout {
			status.Done = true
			status.Error = fmt.Sprintf("Readiness timeout after %s: target %s not ready",
				readinessTimeout, target.Name)
			return
		} else {
			remaining := readinessTimeout - elapsed
			if remaining < minRemaining {
				minRemaining = remaining
			}
		}
	}
	if minRemaining < readinessTimeout {
		if result.RequeueAfter == 0 || result.RequeueAfter > minRemaining {
			result.RequeueAfter = minRemaining
		}
	}
}

// errToString returns a stringified version of an error or an empty string if the error is nil.
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
