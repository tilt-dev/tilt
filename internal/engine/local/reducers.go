package local

import (
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// When the Cmd controller updates a command, check to see
//
// If the local serve cmd is watching the cmd, update
// the local runtime state to match the cmd status.
func HandleCmdUpdateStatusAction(state *store.EngineState, action CmdUpdateStatusAction) {
	cmd, ok := state.Cmds[action.Cmd.Name]
	if !ok {
		return
	}
	cmd = cmd.DeepCopy()
	cmd.Status = action.Cmd.Status
	state.Cmds[action.Cmd.Name] = cmd
	updateLocalRuntimeStatus(state, cmd)
}

// If the local serve cmd is watching the cmd, update
// the local runtime state to match the cmd status.
func updateLocalRuntimeStatus(state *store.EngineState, cmd *v1alpha1.Cmd) {
	mn := model.ManifestName(cmd.Annotations[v1alpha1.AnnotationManifest])
	mt, ok := state.ManifestTargets[mn]
	if !ok {
		delete(state.Cmds, cmd.Name)
		return
	}

	ms := mt.State
	lrs := ms.LocalRuntimeState()
	if lrs.CmdName != cmd.Name {
		return
	}

	state.Cmds[cmd.Name] = cmd

	spec := cmd.Spec
	status := cmd.Status
	if status.Running != nil {
		lrs.PID = int(cmd.Status.Running.PID)
		lrs.StartTime = cmd.Status.Running.StartedAt.Time
		lrs.FinishTime = time.Time{}

		// Currently, Cmd is only used for servers.
		// Make the Status OK when the readiness probe passes (if there is one).
		if spec.ReadinessProbe == nil || cmd.Status.Ready {
			lrs.Status = v1alpha1.RuntimeStatusOK
		} else {
			lrs.Status = v1alpha1.RuntimeStatusPending
		}

	} else if status.Terminated != nil {
		// Currently, CMD is only used for servers,
		// so any termination is an error.
		lrs.PID = int(status.Terminated.PID)
		lrs.Status = v1alpha1.RuntimeStatusError
		lrs.StartTime = status.Terminated.StartedAt.Time
		lrs.FinishTime = status.Terminated.FinishedAt.Time
	} else {
		lrs.Status = v1alpha1.RuntimeStatusPending
		lrs.StartTime = time.Time{}
		lrs.FinishTime = time.Time{}
	}

	if lrs.Ready != cmd.Status.Ready {
		lrs.Ready = cmd.Status.Ready
		if lrs.Ready {
			lrs.LastReadyOrSucceededTime = time.Now()
		}
	}
	lrs.SpanID = model.LogSpanID(cmd.ObjectMeta.Annotations[v1alpha1.AnnotationSpanID])

	ms.RuntimeState = lrs
}

// When the local controller creates a new command, link
// that command to the Local runtime state.
func HandleCmdCreateAction(state *store.EngineState, action CmdCreateAction) {
	cmd := action.Cmd
	mn := model.ManifestName(cmd.Annotations[v1alpha1.AnnotationManifest])
	mt, ok := state.ManifestTargets[mn]
	if !ok {
		return
	}

	ms := mt.State
	lrs := ms.LocalRuntimeState()
	lrs.CmdName = cmd.Name
	ms.RuntimeState = lrs

	updateLocalRuntimeStatus(state, cmd)
}

// Mark the command for deletion.
func HandleCmdDeleteAction(state *store.EngineState, action CmdDeleteAction) {
	delete(state.Cmds, action.Name)
}
