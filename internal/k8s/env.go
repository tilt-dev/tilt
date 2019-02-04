package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
)

type Env string

const (
	EnvUnknown       Env = "unknown"
	EnvGKE               = "gke"
	EnvMinikube          = "minikube"
	EnvDockerDesktop     = "docker-for-desktop"
	EnvMicroK8s          = "microk8s"
	EnvNone              = "none" // k8s not running (not neces. a problem, e.g. if using Tilt x Docker Compose)
)

func (e Env) IsLocalCluster() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s
}

// If something goes wrong getting k8s Env, we want to hold onto err in case we need to
// surface for debugging, but it might be an expected case so we don't want to stop wiring.
type EnvOrError struct {
	Env Env
	Err error
}

func ProvideEnvOrError(ctx context.Context) EnvOrError {
	kubeContext, err := detectKubeContext()
	if err != nil {
		logger.Get(ctx).Debugf(err.Error())
		return EnvOrError{
			Env: EnvNone,
			Err: err,
		}
	}
	return EnvOrError{Env: EnvFromString(string(kubeContext))}
}

func detectKubeContext() (KubeContext, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	outputBytes, err := cmd.Output()

	if err != nil {
		exitErr, isExit := err.(*exec.ExitError)
		if isExit {
			return KubeContext(""), fmt.Errorf("DetectKubeContext failed. Output:\n%s", string(exitErr.Stderr))
		} else {
			return KubeContext(""), errors.Wrap(err, "DetectKubeContext failed")
		}
	}

	output := strings.TrimSpace(string(outputBytes))
	return KubeContext(output), nil
}

func EnvFromString(s string) Env {
	if s == EnvMinikube {
		return EnvMinikube
	} else if s == EnvDockerDesktop || s == "docker-desktop" {
		return EnvDockerDesktop
	} else if s == EnvMicroK8s {
		return EnvMicroK8s
	} else if s == EnvNone {
		return EnvNone
	} else if strings.HasPrefix(s, EnvGKE) {
		// GKE context strings look like:
		// gke_blorg-dev_us-central1-b_blorg
		return EnvGKE
	}
	return EnvUnknown
}

func ProideEnv(envOrErr EnvOrError) Env {
	return envOrErr.Env
}
