package liveupdates

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type UpdateMode string

// A type to bind to flag values that need validation.
type UpdateModeFlag UpdateMode

var (
	// Auto-pick the build mode based on
	UpdateModeAuto UpdateMode = "auto"

	// Only do image builds
	UpdateModeImage UpdateMode = "image"

	// Update containers in-place. This mode only works with DockerForDesktop and Minikube.
	// If you try to use this mode with a different K8s cluster type, we will return an error
	UpdateModeContainer UpdateMode = "container"

	// Use `kubectl exec`
	UpdateModeKubectlExec UpdateMode = "exec"
)

var AllUpdateModes = []UpdateMode{
	UpdateModeAuto,
	UpdateModeImage,
	UpdateModeContainer,
	UpdateModeKubectlExec,
}

func ProvideUpdateMode(flag UpdateModeFlag, kubeContext k8s.KubeContext, env docker.ClusterEnv) (UpdateMode, error) {
	valid := false
	for _, mode := range AllUpdateModes {
		if mode == UpdateMode(flag) {
			valid = true
		}
	}

	if !valid {
		return "", fmt.Errorf("unknown update mode %q. Valid Values: %v", flag, AllUpdateModes)
	}

	mode := UpdateMode(flag)
	if mode == UpdateModeContainer {
		if !docker.Env(env).WillBuildToKubeContext(kubeContext) {
			return "", fmt.Errorf("update mode %q is only valid with local Docker clusters like Docker For Mac or Minikube", flag)
		}
	}

	return mode, nil
}
