package configmap

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func DisableStatus(ctx context.Context, client client.Client, disableSource *v1alpha1.DisableSource) (isDisabled bool, reason string, err error) {
	if disableSource == nil {
		return false, "object does not specify a DisableSource", nil
	}

	if disableSource.ConfigMap == nil {
		return false, "DisableSource specifies no ConfigMap", nil
	}

	cm := &v1alpha1.ConfigMap{}
	err = client.Get(ctx, types.NamespacedName{Name: disableSource.ConfigMap.Name}, cm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, fmt.Sprintf("ConfigMap %q does not exist", disableSource.ConfigMap.Name), nil
		}
		return false, fmt.Sprintf("error reading ConfigMap %q", disableSource.ConfigMap.Name), err
	}

	cmVal, ok := cm.Data[disableSource.ConfigMap.Key]
	if !ok {
		return false, fmt.Sprintf("ConfigMap %q has no key %q", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key), nil
	}
	isDisabled, err = strconv.ParseBool(cmVal)
	if err != nil {
		return false, fmt.Sprintf("error parsing ConfigMap/key %q/%q value %q as a bool: %v", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key, cmVal, err.Error()), nil
	}

	return isDisabled, fmt.Sprintf("ConfigMap/key %q/%q is %v", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key, isDisabled), nil
}

// Returns a new DisableStatus if the disable status has changed, or the prev status if it hasn't.
func MaybeNewDisableStatus(ctx context.Context, client client.Client, disableSource *v1alpha1.DisableSource, prevStatus *v1alpha1.DisableStatus) (*v1alpha1.DisableStatus, error) {
	isDisabled, reason, err := DisableStatus(ctx, client, disableSource)
	if err != nil {
		return nil, err
	}
	statusDiffers := prevStatus == nil || prevStatus.Disabled != isDisabled || prevStatus.Reason != reason
	if statusDiffers {
		return &v1alpha1.DisableStatus{
			Disabled:       isDisabled,
			LastUpdateTime: metav1.Now(),
			Reason:         reason,
		}, nil
	}
	return prevStatus, nil
}
