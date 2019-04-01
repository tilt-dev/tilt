package model

import (
	"github.com/pkg/errors"
)

// Specifies how to update a running container.
// 1. If there are Sync steps in `Steps`, files will be synced as specified.
// 2. Any time we sync one or more files, all Run and RestartContainer steps will be evaluated.
type LiveUpdate struct {
	Steps []LiveUpdateStep

	// When files matching any of these globs change, we should fall back to a full rebuild.
	FullRebuildTriggers Globset
}

func NewLiveUpdate(steps []LiveUpdateStep, fullRebuildTriggers Globset) (LiveUpdate, error) {
	seenRunStep := false
	for i, step := range steps {
		switch step.(type) {
		case LiveUpdateSyncStep:
			if seenRunStep {
				return LiveUpdate{}, errors.New("all sync steps must precede all run steps")
			}
		case LiveUpdateRunStep:
			seenRunStep = true
		case LiveUpdateRestartContainerStep:
			if i != len(steps)-1 {
				return LiveUpdate{}, errors.New("restart container is only valid as the last step")
			}
		}
	}
	return LiveUpdate{steps, fullRebuildTriggers}, nil
}

type LiveUpdateStep interface {
	liveUpdateStep()
}

// Specifies that changes to local path `Source` should be synced to container path `Dest`
type LiveUpdateSyncStep struct {
	Source, Dest string
}

func (l LiveUpdateSyncStep) liveUpdateStep() {}

func (l LiveUpdateSyncStep) toSync() Sync {
	return Sync{
		LocalPath:     l.Source,
		ContainerPath: l.Dest,
	}
}

// Specifies that `Command` should be executed when any files in `Sync` steps have changed
// If `Trigger` is non-empty, `Command` will only be executed when the local paths of changed files covered by
// at least one `Sync` match one of `Globset.Globs` (evaluated relative to `Globset.BaseDirectory`.
type LiveUpdateRunStep struct {
	Command  Cmd
	Triggers Globset
}

func (l LiveUpdateRunStep) liveUpdateStep() {}

func (l LiveUpdateRunStep) toRun() Run {
	return Run{Cmd: l.Command, Triggers: l.Triggers}
}

// Specifies that the container should be restarted when any files in `Sync` steps have changed.
type LiveUpdateRestartContainerStep struct{}

func (l LiveUpdateRestartContainerStep) liveUpdateStep() {}

func (lu LiveUpdate) SyncSteps() []Sync {
	var syncs []Sync
	for _, step := range lu.Steps {
		switch step := step.(type) {
		case LiveUpdateSyncStep:
			syncs = append(syncs, step.toSync())
		}
	}
	return syncs
}

func (lu LiveUpdate) RunSteps() []Run {
	var runs []Run
	for _, step := range lu.Steps {
		switch step := step.(type) {
		case LiveUpdateRunStep:
			runs = append(runs, step.toRun())
		}
	}
	return runs
}

func (lu LiveUpdate) ShouldRestart() bool {
	if len(lu.Steps) > 0 {
		// Currently we require that the Restart step, if present, must be the last step.
		last := lu.Steps[len(lu.Steps)-1]
		if _, ok := last.(LiveUpdateRestartContainerStep); ok {
			return true
		}
	}
	return false
}
