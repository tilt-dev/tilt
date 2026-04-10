package liveupdate

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Each LiveUpdate has a monitor associated with it that
// tracks the history of updates.
//
// The monitor keeps track of:
// - The last known Spec
// - Every file change it has seen
// - The history of container updates
type monitor struct {
	manifestName string
	spec         v1alpha1.LiveUpdateSpec

	// Tracked dependencies.
	lastKubernetesDiscovery   *v1alpha1.KubernetesDiscovery
	lastKubernetesApplyStatus *v1alpha1.KubernetesApplyStatus
	lastDockerComposeService  *v1alpha1.DockerComposeService
	lastTriggerQueue          *v1alpha1.ConfigMap
	lastImageMap              *v1alpha1.ImageMap

	// History of source file changes.
	sources map[string]*monitorSource

	// History of container updates.
	hasChangesToSync bool
	containers       map[monitorContainerKey]monitorContainerStatus
}

type monitorSource struct {
	modTimeByPath   map[string]metav1.MicroTime
	lastImageStatus *v1alpha1.ImageMapStatus
	lastFileEvent   *v1alpha1.FileEvent
}

type monitorContainerKey struct {
	containerID string
	podName     string
	namespace   string
}

// needsInitialSync reports whether any container may still need an initial
// sync. Returns true when no containers are tracked yet (first reconcile,
// or after garbage collection) or when any tracked container has never
// been synced.
func (m *monitor) needsInitialSync() bool {
	if len(m.containers) == 0 {
		return true
	}
	for _, c := range m.containers {
		if c.lastFileTimeSynced.IsZero() {
			return true
		}
	}
	return false
}

type monitorContainerStatus struct {
	lastFileTimeSynced metav1.MicroTime

	// The low water mark is the oldest file timestamp
	// triggered a build failure.
	//
	// If we get a new ImageBuild or new KubernetesApply
	// after this mark, we should try again.
	failedLowWaterMark metav1.MicroTime
	failedReason       string
	failedMessage      string
}
