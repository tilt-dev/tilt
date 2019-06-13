package engine

import "os"

// TODO(maia): remove this when feature is merged

const k8sEventsFeatureFlag = "TILT_K8S_EVENTS"

// TEMPORARY: check env to see if feature flag is set
func k8sEventsFeatureFlagOn() bool {
	return os.Getenv(k8sEventsFeatureFlag) != ""
}
