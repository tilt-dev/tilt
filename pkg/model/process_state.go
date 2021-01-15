package model

// ProcessState represents the current execution state of a local process.
type ProcessState string

const (
	// ProcessStateRunning indicates that a process is alive and executing.
	ProcessStateRunning ProcessState = "running"
	// ProcessStateTerminated indicates that a process is no longer executing
	// either because it exited or was terminated.
	ProcessStateTerminated ProcessState = "terminated"
)
