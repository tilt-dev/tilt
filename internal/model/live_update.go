package model

import "github.com/pkg/errors"

// Specifies how to update a running container.
// 1. If there are Sync steps in `Steps`, files will be synced as specified.
// 2. Any time we sync one or more files, all Run and RestartContainer steps will be evaluated.
type LiveUpdate struct {
	Steps []LiveUpdateStep

	// When files matching any of these globs change, we should fall back to a full rebuild.
	FullRebuildTriggers []string
}

func NewLiveUpdate(steps []LiveUpdateStep, fullRebuildTriggers []string) (LiveUpdate, error) {
	seenRunStep := false
	for i, step := range steps {
		switch step.(type) {
		case LiveUpdateWorkDirStep:
			if i != 0 {
				return LiveUpdate{}, errors.New("workdir is only valid as the first step")
			}
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

// Specifies the in-container directory from which `Run` steps are executed
type LiveUpdateWorkDirStep string

func (l LiveUpdateWorkDirStep) liveUpdateStep() {}

// Specifies that changes to local path `Source` should be synced to container path `Dest`
type LiveUpdateSyncStep struct {
	Source, Dest string
}

func (l LiveUpdateSyncStep) liveUpdateStep() {}

// Specifies that `Command` should be executed when any files in `Sync` steps have changed
// If `Trigger` is non-empty, `Command` will only be executed when the local paths of changed files covered by
// at least one `Sync` match the glob in `Trigger`.
type LiveUpdateRunStep struct {
	Command  Cmd
	Triggers []string
}

func (l LiveUpdateRunStep) liveUpdateStep() {}

// Specifies that the container should be restarted when any files in `Sync` steps have changed.
type LiveUpdateRestartContainerStep struct{}

func (l LiveUpdateRestartContainerStep) liveUpdateStep() {}
