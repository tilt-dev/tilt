package cmdimage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Return a basic Building Status.
func ToBuildingStatus(iTarget model.ImageTarget, startTime metav1.MicroTime) v1alpha1.CmdImageStatus {
	return v1alpha1.CmdImageStatus{
		Building: &v1alpha1.CmdImageStateBuilding{
			StartedAt: startTime,
		},
	}
}

// Return a completed status when the image build failed.
func ToCompletedFailStatus(iTarget model.ImageTarget, startTime metav1.MicroTime,
	err error) v1alpha1.CmdImageStatus {
	finishTime := apis.NowMicro()
	return v1alpha1.CmdImageStatus{
		Completed: &v1alpha1.CmdImageStateCompleted{
			StartedAt:  startTime,
			FinishedAt: finishTime,
			Error:      err.Error(),
		},
	}
}

// Return a completed status when the image build succeeded.
func ToCompletedSuccessStatus(iTarget model.ImageTarget, startTime metav1.MicroTime,
	refs container.TaggedRefs) v1alpha1.CmdImageStatus {
	finishTime := apis.NowMicro()
	return v1alpha1.CmdImageStatus{
		Ref: container.FamiliarString(refs.LocalRef),
		Completed: &v1alpha1.CmdImageStateCompleted{
			StartedAt:  startTime,
			FinishedAt: finishTime,
		},
	}
}
