package docker

import (
	"time"

	"github.com/docker/docker/api/types"
)

// Helper functions for generating container states.

func NewCreatedContainerState() types.ContainerState {
	return types.ContainerState{
		Status:     "created",
		StartedAt:  ZeroTime,
		FinishedAt: ZeroTime,
	}
}

func NewRunningContainerState() types.ContainerState {
	return types.ContainerState{
		Running:    true,
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: ZeroTime,
	}
}

func NewExitSuccessContainerState() types.ContainerState {
	return types.ContainerState{
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
		ExitCode:   0,
	}
}

func NewExitErrorContainerState() types.ContainerState {
	return types.ContainerState{
		StartedAt:  time.Now().Format(time.RFC3339Nano),
		FinishedAt: time.Now().Format(time.RFC3339Nano),
		ExitCode:   1,
	}
}
