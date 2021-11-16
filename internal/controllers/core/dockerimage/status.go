package dockerimage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Return a basic Building Status.
func ToBuildingStatus(iTarget model.ImageTarget, startTime metav1.MicroTime) v1alpha1.DockerImageStatus {
	return v1alpha1.DockerImageStatus{
		Building: &v1alpha1.DockerImageStateBuilding{
			StartedAt: startTime,
		},
	}
}

// Return a completed status when the image build failed.
func ToCompletedFailStatus(iTarget model.ImageTarget, startTime metav1.MicroTime,
	stages []v1alpha1.DockerImageStageStatus, err error) v1alpha1.DockerImageStatus {
	finishTime := apis.NowMicro()

	// Complete all stages.
	for i, stage := range stages {
		if stage.StartedAt != nil && stage.FinishedAt == nil {
			stage.FinishedAt = &finishTime
			stage.Error = err.Error()
		}
		stages[i] = stage
	}

	return v1alpha1.DockerImageStatus{
		Completed: &v1alpha1.DockerImageStateCompleted{
			StartedAt:  startTime,
			FinishedAt: finishTime,
			Error:      err.Error(),
		},
		StageStatuses: stages,
	}
}

// Return a completed status when the image build succeeded.
func ToCompletedSuccessStatus(iTarget model.ImageTarget, startTime metav1.MicroTime,
	stages []v1alpha1.DockerImageStageStatus, refs container.TaggedRefs) v1alpha1.DockerImageStatus {
	finishTime := apis.NowMicro()

	// Complete all stages.
	for i, stage := range stages {
		if stage.StartedAt != nil && stage.FinishedAt == nil {
			stage.FinishedAt = &finishTime
		}
		stages[i] = stage
	}

	return v1alpha1.DockerImageStatus{
		Ref: container.FamiliarString(refs.LocalRef),
		Completed: &v1alpha1.DockerImageStateCompleted{
			StartedAt:  startTime,
			FinishedAt: finishTime,
		},
		StageStatuses: stages,
	}
}
