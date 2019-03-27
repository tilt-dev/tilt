package model

type LiveUpdate struct {
	Steps []LiveUpdateStep

	// When files matching any of these globs change, we should fall back to a full rebuild.
	FullRebuildTriggers []string
}

type LiveUpdateStep interface {
	liveUpdateStep()
}

// Specifies the in-container directory from which `Run` steps are executed
type LiveUpdateWorkDirStep string

func (l LiveUpdateWorkDirStep) liveUpdateStep() {}

// Specifies that when there are changes to the local path `Source`, we:
// 1. Sync those changes to the remote path `Dest`
// 2. Trigger any applicable `Run` steps
type LiveUpdateSyncStep struct {
	Source, Dest string
}

func (l LiveUpdateSyncStep) liveUpdateStep() {}

// Specifies that `Command` should be executed when any files in `Sync` steps have changed
// If `Trigger` is non-empty, will only be executed when those changes match the glob specified in `Trigger`.
type LiveUpdateRunStep struct {
	Command Cmd
	Trigger string
}

func (l LiveUpdateRunStep) liveUpdateStep() {}

// Specifies that any files matched by `Sync` steps should also cause the container to be restarted.
type LiveUpdateRestartContainerStep struct{}

func (l LiveUpdateRestartContainerStep) LiveUpdateStep() {}
