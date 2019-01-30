package k8s

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

type Env string

const (
	EnvUnknown       Env = "unknown"
	EnvGKE               = "gke"
	EnvMinikube          = "minikube"
	EnvDockerDesktop     = "docker-for-desktop"
	EnvMicroK8s          = "microk8s"
)

func (e Env) IsLocalCluster() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s
}

func DetectEnv(kubeContext KubeContext) (Env, error) {
	return EnvFromString(string(kubeContext)), nil
}

func DetectKubeContext() (KubeContext, error) {
	cmd := exec.Command("kubectl", "config", "current-context")
	outputBytes, err := cmd.Output()

	if err != nil {
		exitErr, isExit := err.(*exec.ExitError)
		if isExit {
			return KubeContext(""), fmt.Errorf("DetectKubeContext failed. Output:\n%s", string(exitErr.Stderr))
		}
		return KubeContext(""), errors.Wrap(err, "DetectKubeContext")
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
	} else if strings.HasPrefix(s, EnvGKE) {
		// GKE context strings look like:
		// gke_blorg-dev_us-central1-b_blorg
		return EnvGKE
	}
	return EnvUnknown
}
