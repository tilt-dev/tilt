package configmap

import (
	"context"
	"fmt"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type DisableResult int

func DisableStatus(getCM func(name string) (v1alpha1.ConfigMap, error), disableSource *v1alpha1.DisableSource) (result v1alpha1.DisableState, reason string, err error) {
	if disableSource == nil {
		// if there is no source, assume the object has opted out of being disabled and is always eanbled
		return v1alpha1.DisableStateEnabled, "object does not specify a DisableSource", nil
	}

	if disableSource.ConfigMap == nil {
		return v1alpha1.DisableStateError, "DisableSource specifies no ConfigMap", nil
	}

	cm, err := getCM(disableSource.ConfigMap.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return v1alpha1.DisableStatePending, fmt.Sprintf("ConfigMap %q does not exist", disableSource.ConfigMap.Name), nil
		}
		return v1alpha1.DisableStatePending, fmt.Sprintf("error reading ConfigMap %q", disableSource.ConfigMap.Name), err
	}

	cmVal, ok := cm.Data[disableSource.ConfigMap.Key]
	if !ok {
		return v1alpha1.DisableStateError, fmt.Sprintf("ConfigMap %q has no key %q", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key), nil
	}
	isDisabled, err := strconv.ParseBool(cmVal)
	if err != nil {
		return v1alpha1.DisableStateError, fmt.Sprintf("error parsing ConfigMap/key %q/%q value %q as a bool: %v", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key, cmVal, err.Error()), nil
	}

	if isDisabled {
		result = v1alpha1.DisableStateDisabled
	} else {
		result = v1alpha1.DisableStateEnabled
	}
	return result, fmt.Sprintf("ConfigMap/key %q/%q is %v", disableSource.ConfigMap.Name, disableSource.ConfigMap.Key, isDisabled), nil
}

// Returns a new DisableStatus if the disable status has changed, or the prev status if it hasn't.
func MaybeNewDisableStatus(ctx context.Context, client client.Client, disableSource *v1alpha1.DisableSource, prevStatus *v1alpha1.DisableStatus) (*v1alpha1.DisableStatus, error) {
	getCM := func(name string) (v1alpha1.ConfigMap, error) {
		var cm v1alpha1.ConfigMap
		err := client.Get(ctx, types.NamespacedName{Name: name}, &cm)
		return cm, err
	}

	result, reason, err := DisableStatus(getCM, disableSource)
	if err != nil {
		return nil, err
	}
	// we treat pending as disabled
	// eventually we should probably represent isDisabled by an enum in the API, but for now
	// we treat pending as disabled, with the understanding that it's better to momentarily delay the start of an
	// object than to spin it up and quickly kill it, as the latter might generate undesired side effects / logs
	isDisabled := result == v1alpha1.DisableStateDisabled || result == v1alpha1.DisableStatePending || result == v1alpha1.DisableStateError
	statusDiffers := prevStatus == nil || prevStatus.State != result || prevStatus.Reason != reason
	if statusDiffers {
		return &v1alpha1.DisableStatus{
			Disabled:       isDisabled,
			LastUpdateTime: apis.Now(),
			Reason:         reason,
			State:          result,
		}, nil
	}
	return prevStatus, nil
}
