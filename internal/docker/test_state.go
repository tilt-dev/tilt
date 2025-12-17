package docker

import (
	"time"

	typescontainer "github.com/docker/docker/api/types/container"
)

// Helper functions for generating container states.

func NewCreatedContainerState() typescontainer.State {
	return typescontainer.State{
		Status:     "created",
		StartedAt:  ZeroTime,
		FinishedAt: ZeroTime,
	}
}

func NewRunningContainerState() typescontainer.State {
	return typescontainer.State{
		Running:    true,
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: ZeroTime,
	}
}

func NewExitSuccessContainerState() typescontainer.State {
	return typescontainer.State{
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
		ExitCode:   0,
	}
}

func NewExitErrorContainerState() typescontainer.State {
	return typescontainer.State{
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
		ExitCode:   1,
	}
}
