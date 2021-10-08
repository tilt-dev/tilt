package liveupdate

import (
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Input struct {
	// TODO(nick|milas): Figure out what should happen with this filter.11
	Filter model.PathMatcher

	// Derived from DockerResource
	IsDC bool

	// Derived from KubernetesResource + KubenetesSelector + DockerResource
	Containers []liveupdates.Container

	// Derived from FileWatch + Sync rules
	ChangedFiles []build.PathMapping
}
