package k8s

import (
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
)

type Env string

const (
	EnvUnknown       Env = "unknown"
	EnvGKE           Env = "gke"
	EnvMinikube      Env = "minikube"
	EnvDockerDesktop Env = "docker-for-desktop"
	EnvMicroK8s      Env = "microk8s"
	EnvKIND          Env = "kind"
	EnvNone          Env = "none" // k8s not running (not neces. a problem, e.g. if using Tilt x Docker Compose)
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
	if Env(s) == EnvMinikube {
		return EnvMinikube
	} else if Env(s) == EnvDockerDesktop || s == "docker-desktop" {
		return EnvDockerDesktop
	} else if Env(s) == EnvMicroK8s {
		return EnvMicroK8s
	} else if strings.HasPrefix(s, "kubernetes-admin@kind") {
		return EnvKIND
	} else if Env(s) == EnvNone {
		return EnvNone
	} else if strings.HasPrefix(s, string(EnvGKE)) {
		// GKE context strings look like:
		// gke_blorg-dev_us-central1-b_blorg
		return EnvGKE
	}
	return EnvUnknown
}
