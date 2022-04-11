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
