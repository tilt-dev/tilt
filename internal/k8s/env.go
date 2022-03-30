package k8s

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/clusterid"
)

type ClusterName string

func ProvideKubeContext(config *api.Config) (KubeContext, error) {
	return KubeContext(config.CurrentContext), nil
}

func ProvideKubeConfig(clientLoader clientcmd.ClientConfig, contextOverride KubeContextOverride) (*api.Config, error) {
	config, err := clientLoader.RawConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Loading Kubernetes current-context")
	}

	// NOTE(nick): The RawConfig() accessor doesn't handle overrides.
	// The other accessors do. So we do what ClientConfig does internally, and
	// apply the overrides ourselves.
	if contextOverride != "" {
		config.CurrentContext = string(contextOverride)

		// If the user explicitly passed an override, validate it.
		err := clientcmd.ConfirmUsable(config, string(contextOverride))
		if err != nil {
			return nil, errors.Wrap(err, "Overriding Kubernetes context")
		}
	}

	return &config, nil
}

func ProvideClusterName(config *api.Config) ClusterName {
	n := config.CurrentContext
	c, ok := config.Contexts[n]
	if !ok {
		return ""
	}
	return ClusterName(c.Cluster)
}

const ProductNone = clusterid.Product("")

func ProvideClusterProduct(ctx context.Context, config *api.Config) clusterid.Product {
	n := config.CurrentContext

	c, ok := config.Contexts[n]
	if !ok {
		if n == "" {
			return ProductNone
		}
		return clusterid.ProductUnknown
	}

	cn := c.Cluster
	cl := config.Clusters[cn]
	return clusterid.ProductFromContext(c, cl)
}

// Convert the current cluster type to an analytics env, for backwards compatibility.
func AnalyticsEnv(p clusterid.Product) string {
	if p == clusterid.ProductDockerDesktop {
		return "docker-for-desktop"
	}
	if p == clusterid.ProductKIND {
		return "kind-0.6+"
	}
	return string(p)
}
