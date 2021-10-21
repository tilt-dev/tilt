package liveupdate

import (
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
)

type Input struct {
	// Derived from DockerResource
	IsDC bool

	// Derived from KubernetesResource + KubenetesSelector + DockerResource
	Containers []liveupdates.Container

	// Derived from FileWatch + Sync rules
	ChangedFiles []build.PathMapping
}
