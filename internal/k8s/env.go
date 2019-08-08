package k8s

import (
	"strings"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
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

func (e Env) UsesLocalDockerRegistry() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s
}

func (e Env) IsLocalCluster() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s || e == EnvKIND
}

func ProvideEnv(kubeConfig *api.Config) Env {
	return EnvFromConfig(kubeConfig)
}

func ProvideKubeContext(config *api.Config) (KubeContext, error) {
	return KubeContext(config.CurrentContext), nil
}

func ProvideKubeConfig(clientLoader clientcmd.ClientConfig) (*api.Config, error) {
	access := clientLoader.ConfigAccess()
	config, err := access.GetStartingConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Loading Kubernetes current-context")
	}

	return config, nil
}

func EnvFromConfig(config *api.Config) Env {
	n := config.CurrentContext

	c, ok := config.Contexts[n]
	if !ok {
		return EnvUnknown
	}

	cn := c.Cluster
	if Env(cn) == EnvMinikube {
		return EnvMinikube
	} else if cn == "docker-for-desktop-cluster" || cn == "docker-desktop" {
		return EnvDockerDesktop
	} else if strings.HasPrefix(cn, string(EnvGKE)) {
		// GKE cluster strings look like:
		// gke_blorg-dev_us-central1-b_blorg
		return EnvGKE
	} else if Env(cn) == EnvKIND {
		return EnvKIND
	} else if cn == "microk8s-cluster" {
		return EnvMicroK8s
	}

	return EnvUnknown
}
