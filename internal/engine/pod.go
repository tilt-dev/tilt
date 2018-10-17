package engine

import (
	"github.com/google/uuid"
	k8s "github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"k8s.io/api/core/v1"
)

const TiltRunIDLabel = "tilt-runid"

var TiltRunID = uuid.New().String()

const ManifestNameLabel = "tilt-manifest"

func TiltRunLabel() k8s.LabelPair {
	return k8s.LabelPair{
		Key:   TiltRunIDLabel,
		Value: TiltRunID,
	}
}

func RestartsForPod(pod *v1.Pod) int {
	var restarts int32 = 0
	for _, cStatus := range pod.Status.ContainerStatuses {
		if cStatus.Name == sidecar.SyncletContainerName {
			continue
		}
		// If pod has multiple containers, sum their restarts (same behavior as kubectl get pods)
		if cStatus.RestartCount > restarts {
			restarts += cStatus.RestartCount
		}
	}
	return int(restarts)
}
