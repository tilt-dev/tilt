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

	// InitialSync is set during initial sync to enable tar batching directly from
	// the configured sync mappings instead of individual file PathMappings.
	InitialSync bool

	// InitialSyncFilter applies the same ignore semantics used by the source
	// FileWatch objects during the initial filesystem walk and tar creation.
	InitialSyncFilter model.PathMatcher
}
