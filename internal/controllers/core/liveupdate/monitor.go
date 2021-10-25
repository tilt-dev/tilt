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
	spec v1alpha1.LiveUpdateSpec

	// Tracked dependencies.
	lastKubernetesDiscovery   *v1alpha1.KubernetesDiscovery
	lastKubernetesApplyStatus *v1alpha1.KubernetesApplyStatus
	lastImageMapStatus        *v1alpha1.ImageMapStatus
	lastFileEvents            map[string]*v1alpha1.FileEvent

	// History of file changes.
	modTimeByPath map[string]metav1.MicroTime

	// History of container updates.
	hasChangesToSync bool
	containers       map[monitorContainerKey]monitorContainerStatus
}

type monitorContainerKey struct {
	containerID string
	podName     string
	namespace   string
}

type monitorContainerStatus struct {
	lastFileTimeSynced metav1.MicroTime

	// Once a container is marked unrecoverable,
	// we never send updates to it again.
	failedReason  string
	failedMessage string
}
