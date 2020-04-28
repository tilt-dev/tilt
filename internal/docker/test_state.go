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
		StartedAt:  time.Now().String(),
		FinishedAt: ZeroTime,
	}
}

func NewExitSuccessContainerState() types.ContainerState {
	return types.ContainerState{
		StartedAt:  time.Now().String(),
		FinishedAt: time.Now().String(),
		ExitCode:   0,
	}
}

func NewExitErrorContainerState() types.ContainerState {
	return types.ContainerState{
		StartedAt:  time.Now().String(),
		FinishedAt: time.Now().String(),
		ExitCode:   1,
	}
}
