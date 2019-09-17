package k8s

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/pkg/logger"
)

type ClusterName string
type Env string

const (
	EnvUnknown       Env = "unknown"
	EnvGKE           Env = "gke"
	EnvMinikube      Env = "minikube"
	EnvDockerDesktop Env = "docker-for-desktop"
	EnvMicroK8s      Env = "microk8s"
	EnvKIND          Env = "kind"
	EnvK3D           Env = "k3d"
	EnvNone          Env = "none" // k8s not running (not neces. a problem, e.g. if using Tilt x Docker Compose)
)

func (e Env) UsesLocalDockerRegistry() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s
}

func (e Env) IsLocalCluster() bool {
	return e == EnvMinikube || e == EnvDockerDesktop || e == EnvMicroK8s || e == EnvKIND || e == EnvK3D
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

func ProvideClusterName(ctx context.Context, config *api.Config) ClusterName {
	n := config.CurrentContext
	c, ok := config.Contexts[n]
	if !ok {
		return ""
	}
	return ClusterName(c.Cluster)
}

func ProvideEnv(ctx context.Context, config *api.Config) Env {
	n := config.CurrentContext

	c, ok := config.Contexts[n]
	if !ok {
		if n == "" {
			return EnvNone
		}
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

	loc := c.LocationOfOrigin
	homedir, err := homedir.Dir()
	if err != nil {
		logger.Get(ctx).Infof("Error loading homedir: %v", err)
		return EnvUnknown
	}

	k3dDir := filepath.Join(homedir, ".config", "k3d")
	if ospath.IsChild(k3dDir, loc) {
		return EnvK3D
	}

	// NOTE(nick): Users can set the KIND cluster name with `kind create cluster
	// --name`.  This makes the KIND cluster really hard to detect.
	//
	// We currently do it by assuming that KIND configs are always stored in a
	// file named kind-config-*.
	//
	// KIND internally looks for its clusters with `docker ps` + filters,
	// which might be a route to explore if this isn't robust enough.
	if strings.HasPrefix(filepath.Base(loc), "kind-config-") {
		return EnvKIND
	}

	return EnvUnknown
}
