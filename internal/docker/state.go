package docker

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

// Helper functions for dealing with ContainerState.
const ZeroTime = "0001-01-01T00:00:00Z"

func HasStarted(cState v1alpha1.DockerContainerState) bool {
	return !cState.StartedAt.IsZero()
}

func HasFinished(cState v1alpha1.DockerContainerState) bool {
	return !cState.FinishedAt.IsZero()
}
