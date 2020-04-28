package docker

import "github.com/docker/docker/api/types"

// Helper functions for dealing with ContainerState.
const ZeroTime = "0001-01-01T00:00:00Z"

func HasStarted(cState types.ContainerState) bool {
	return cState.StartedAt != "" && cState.StartedAt != ZeroTime
}

func HasFinished(cState types.ContainerState) bool {
	return cState.FinishedAt != "" && cState.FinishedAt != ZeroTime
}
