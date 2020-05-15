package exit

import (
	"context"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Controls normal process termination. Either Tilt completed all its work,
// or it determined that it was unable to complete the work it was assigned.
type Controller struct {
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) shouldExit(store store.RStore) Action {
	state := store.RLockState()
	defer store.RUnlockState()

	// Already processing the exit
	if state.ExitSignal {
		return Action{}
	}

	if state.EngineMode.IsApplyMode() || state.EngineMode.IsCIMode() {
		// If the tiltfile failed, exit immediately.
		err := state.TiltfileState.LastBuild().Error
		if err != nil {
			return Action{ExitSignal: true, ExitError: err}
		}

		// If any of the individual builds failed, exit immediately.
		for _, mt := range state.ManifestTargets {
			err := mt.State.LastBuild().Error
			if err != nil {
				return Action{ExitSignal: true, ExitError: err}
			}
		}
	}

	if state.EngineMode.IsApplyMode() {
		// If all builds completed, we're done!
		if len(state.ManifestTargets) > 0 && state.InitialBuildsCompleted() {
			return Action{ExitSignal: true}
		}
	}

	if state.EngineMode.IsCIMode() {
		// Check the runtime state of all resources.
		// If any of the resources are in error, exit.
		allOK := true
		for _, mt := range state.ManifestTargets {
			rs := mt.State.RuntimeState
			if rs == nil {
				allOK = false
				continue
			}

			status := rs.RuntimeStatus()
			if status == model.RuntimeStatusError {
				return Action{
					ExitSignal: true,
					ExitError:  rs.RuntimeStatusError(),
				}
			}

			if !c.isRuntimeDone(mt) {
				allOK = false
			}
		}

		// If all the resources are OK, we're done.
		if len(state.ManifestTargets) > 0 &&
			state.InitialBuildsCompleted() && allOK {
			return Action{ExitSignal: true}
		}
	}

	return Action{}
}

func (c *Controller) isRuntimeDone(mt *store.ManifestTarget) bool {
	rs := mt.State.RuntimeState
	if rs == nil {
		return false
	}

	status := rs.RuntimeStatus()
	statusOK := status == model.RuntimeStatusOK || status == model.RuntimeStatusNotApplicable
	if !statusOK {
		return false
	}

	// If this is a job, check to see if it has run to completion
	//
	// TODO(nick): This is...not great. Ideally, Tilt would track the status of
	// every resource type it deploys, then we'd have some general-purpose system
	// for expressing success criteria on different resource types (like
	// https://www.openpolicyagent.org/). This is just a hack to make this work
	// for jobs, until it makes sense to build out that type-independent
	// infrastructure.
	isK8s := mt.Manifest.IsK8s()
	isK8sJob := isK8s && mt.Manifest.K8sTarget().HasJob()
	if isK8sJob {
		k8sState, ok := mt.State.RuntimeState.(store.K8sRuntimeState)
		if !ok {
			return false
		}

		pod := k8sState.MostRecentPod()
		if pod.Phase != v1.PodSucceeded {
			return false
		}
	}

	return true
}

func (c *Controller) OnChange(ctx context.Context, store store.RStore) {
	action := c.shouldExit(store)
	if action.ExitSignal {
		store.Dispatch(action)
	}
}

var _ store.Subscriber = &Controller{}
