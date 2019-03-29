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

// TODO(maia): s/Mount/Sync
func (l LiveUpdateSyncStep) toMount() Mount {
	return Mount{
		LocalPath:     l.Source,
		ContainerPath: l.Dest,
	}
}

// Specifies that `Command` should be executed when any files in `Sync` steps have changed
// If `Trigger` is non-empty, `Command` will only be executed when the local paths of changed files covered by
// at least one `Sync` match the glob in `Trigger`.
type LiveUpdateRunStep struct {
	Command  Cmd
	Triggers []string
	// if non-empty, the remote directory from which to run `Command`
	WorkDir string
}

func (l LiveUpdateRunStep) liveUpdateStep() {}

func (l LiveUpdateRunStep) toRun() Run {
	return Run{Cmd: l.Command, Triggers: l.Triggers}
}

// Specifies that the container should be restarted when any files in `Sync` steps have changed.
type LiveUpdateRestartContainerStep struct{}

func (l LiveUpdateRestartContainerStep) liveUpdateStep() {}

// TODO(maia): s/Mount/Sync
func (lu LiveUpdate) SyncSteps() []Mount {
	var syncs []Mount
	for _, step := range lu.Steps {
		switch step := step.(type) {
		case LiveUpdateSyncStep:
			syncs = append(syncs, step.toMount())
		}
	}
	return syncs
}

func (lu LiveUpdate) RunSteps() []Run {
	// TODO(maia): populate run.BaseDirectory with Workdir (if given)
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
