package model

import (
	"github.com/pkg/errors"
)

// Specifies how to update a running container.
// 1. If there are Sync steps in `Steps`, files will be synced as specified.
// 2. Any time we sync one or more files, all Run and RestartContainer steps will be evaluated.
type LiveUpdate struct {
	Steps []LiveUpdateStep

	// When files matching any of these paths change, we should fall back to a full rebuild.
	FullRebuildTriggers PathSet
}

func NewLiveUpdate(steps []LiveUpdateStep, fullRebuildTriggers PathSet) (LiveUpdate, error) {
	seenRunStep := false
	for i, step := range steps {
		switch step.(type) {
		case LiveUpdateSyncStep:
			if seenRunStep {
				return LiveUpdate{}, errors.New("live_update: all sync steps must precede all run steps")
			}
		case LiveUpdateRunStep:
			seenRunStep = true
		case LiveUpdateRestartContainerStep:
			if i != len(steps)-1 {
				return LiveUpdate{}, errors.New("live_update: restart container is only valid as the last step")
			}
		}
	}
	return LiveUpdate{steps, fullRebuildTriggers}, nil
}

type LiveUpdateStep interface {
	liveUpdateStep()
}

// Specifies that changes to any of the given files should cause the builder to fall back (i.e. do a full image build)
type LiveUpdateFallBackOnStep struct {
	Files []string
}

func (l LiveUpdateFallBackOnStep) liveUpdateStep() {}

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
// at least one `Sync` match one of `PathSet.Paths` (evaluated relative to `PathSet.BaseDirectory`.
type LiveUpdateRunStep struct {
	Command  Cmd
	Triggers PathSet
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
