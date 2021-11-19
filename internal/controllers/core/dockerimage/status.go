package dockerimage

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Write the image status to the API server, if necessary.
// If the image write fails, log it to the debug logs and move on.
func MaybeUpdateStatus(ctx context.Context, ctrlClient ctrlclient.Client,
	iTarget model.ImageTarget, status v1alpha1.DockerImageStatus) {
	if iTarget.DockerImageName == "" {
		return
	}

	nn := types.NamespacedName{Name: iTarget.DockerImageName}
	var di v1alpha1.DockerImage
	err := ctrlClient.Get(ctx, nn, &di)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		logger.Get(ctx).Debugf("fetching dockerimage %s: %v", nn.Name, err)
		return
	}

	if apicmp.DeepEqual(status, di.Status) {
		return
	}

	updated := di.DeepCopy()
	updated.Status = status
	err = ctrlClient.Status().Update(ctx, updated)
	if err != nil {
		logger.Get(ctx).Debugf("updating dockerimage %s: %v", nn.Name, err)
	}
}

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
