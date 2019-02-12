package k8s

import (
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
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

func ProvideEnv(kubeContext KubeContext) Env {
	return EnvFromString(string(kubeContext))
}

func ProvideKubeContext(clientLoader clientcmd.ClientConfig) (KubeContext, error) {
	access := clientLoader.ConfigAccess()
	config, err := access.GetStartingConfig()
	if err != nil {
		return "", errors.Wrap(err, "Loading Kubernetes current-context")
	}
	return KubeContext(config.CurrentContext), nil
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
