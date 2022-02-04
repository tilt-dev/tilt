package uiresource

import (
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func DisableResourceStatus(getCM func(name string) (v1alpha1.ConfigMap, error), disableSources []v1alpha1.DisableSource) (v1alpha1.DisableResourceStatus, error) {
	var result v1alpha1.DisableResourceStatus
	if len(disableSources) == 0 {
		result.State = v1alpha1.DisableStateEnabled
		return result, nil
	}

	errorCount := 0
	pendingCount := 0
	for _, source := range disableSources {

		dr, _, err := configmap.DisableStatus(getCM, &source)
		if err != nil {
			return v1alpha1.DisableResourceStatus{}, err
		}
		switch dr {
		case v1alpha1.DisableStateEnabled:
			result.EnabledCount += 1
		case v1alpha1.DisableStateDisabled:
			result.DisabledCount += 1
		case v1alpha1.DisableStatePending:
			pendingCount += 1
		case v1alpha1.DisableStateError:
			// TODO(matt) - there are arguments for incrementing any of the three fields in this case, or adding an ErrorCount field
			// but none seem compelling. We could add an ErrorCount later, but that's just another case to handle downstream.
			result.DisabledCount += 1
			errorCount += 1
		}
	}
	result.State = v1alpha1.DisableStatePending

	if errorCount > 0 {
		result.State = v1alpha1.DisableStateError
	} else if pendingCount > 0 {
		result.State = v1alpha1.DisableStatePending
	} else if result.DisabledCount > 0 {
		result.State = v1alpha1.DisableStateDisabled
	} else if result.EnabledCount > 0 {
		result.State = v1alpha1.DisableStateEnabled
	}
	result.Sources = disableSources
	return result, nil
}
