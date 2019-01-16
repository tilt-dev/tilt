package engine

import (
	"fmt"

	k8s "github.com/windmilleng/tilt/internal/k8s"
)

type UpdateMode string

// A type to bind to flag values that need validation.
type UpdateModeFlag UpdateMode

var (
	// Auto-pick the build mode based on
	UpdateModeAuto UpdateMode = "auto"

	// Only do image builds
	UpdateModeImage UpdateMode = "image"

	// Only do image builds from scratch
	UpdateModeNaive UpdateMode = "naive"

	// Deploy a synclet to make container updates faster
	UpdateModeSynclet UpdateMode = "synclet"

	// Update containers in-place. This mode only works with DockerForDesktop and Minikube.
	// If you try to use this mode with a different K8s cluster type, we will return an error
	UpdateModeContainer UpdateMode = "container"
)

var AllUpdateModes = []UpdateMode{
	UpdateModeAuto,
	UpdateModeImage,
	UpdateModeNaive,
	UpdateModeSynclet,
	UpdateModeContainer,
}

func ProvideUpdateMode(flag UpdateModeFlag, env k8s.Env) (UpdateMode, error) {
	valid := false
	for _, mode := range AllUpdateModes {
		if mode == UpdateMode(flag) {
			valid = true
		}
	}

	if !valid {
		return "", fmt.Errorf("Unknown update mode %q. Valid Values: %v", flag, AllUpdateModes)
	}

	mode := UpdateMode(flag)
	if mode == UpdateModeContainer && !env.IsLocalCluster() {
		return "", fmt.Errorf("Update mode %q is only valid with local clusters like Docker For Mac, Minikube, and MicroK8s", flag)
	}

	return mode, nil
}
