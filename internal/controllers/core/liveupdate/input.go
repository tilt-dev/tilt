package liveupdate

import (
	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Input struct {
	// Derived from DockerResource
	IsDC bool

	// Derived from KubernetesResource + KubernetesSelector + DockerResource
	Containers []liveupdates.Container

	// Derived from FileWatch + Sync rules
	ChangedFiles []build.PathMapping

	LastFileTimeSynced metav1.MicroTime

	// InitialSyncFilter is set during initial sync to enable tar batching.
	// When set, the archive is built directly from directory-level sync mappings
	// with this filter applied, instead of archiving individual file PathMappings.
	InitialSyncFilter model.PathMatcher
}
