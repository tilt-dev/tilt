package cmdimage

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
	iTarget model.ImageTarget, status v1alpha1.CmdImageStatus) {
	if iTarget.CmdImageName == "" {
		return
	}

	nn := types.NamespacedName{Name: iTarget.CmdImageName}
	var di v1alpha1.CmdImage
	err := ctrlClient.Get(ctx, nn, &di)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		logger.Get(ctx).Debugf("fetching cmdimage %s: %v", nn.Name, err)
		return
	}

	if apicmp.DeepEqual(status, di.Status) {
		return
	}

	updated := di.DeepCopy()
	updated.Status = status
	err = ctrlClient.Status().Update(ctx, updated)
	if err != nil {
		logger.Get(ctx).Debugf("updating cmdimage %s: %v", nn.Name, err)
	}
}

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
