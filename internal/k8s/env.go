package k8s

import (
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/tilt-dev/clusterid"
)

type ClusterName string

func ProvideKubeContext(configOrError APIConfigOrError) KubeContext {
	config := configOrError.Config
	if config == nil {
		return ""
	}
	return KubeContext(config.CurrentContext)
}

type APIConfigOrError struct {
	Config *api.Config
	Error  error
}

func ProvideAPIConfig(clientLoader clientcmd.ClientConfig, contextOverride KubeContextOverride, namespaceOverride NamespaceOverride) APIConfigOrError {
	config, err := clientLoader.RawConfig()
	if err != nil {
		return APIConfigOrError{Error: errors.Wrap(err, "Loading Kubernetes config")}
	}

	// NOTE(nick): The RawConfig() accessor doesn't handle overrides.
	// The other accessors do. So we do what ClientConfig does internally, and
	// apply the overrides ourselves.
	if contextOverride != "" {
		config.CurrentContext = string(contextOverride)
	}

	if namespaceOverride != "" {
		context, ok := config.Contexts[config.CurrentContext]
		if ok {
			context.Namespace = string(namespaceOverride)
		}
	}

	// Use ClientConfig() to workaround bugs in the validation api.
	// See: https://github.com/tilt-dev/tilt/issues/5831
	_, err = clientcmd.NewDefaultClientConfig(config, nil).ClientConfig()
	if err != nil {
		return APIConfigOrError{Error: errors.Wrap(err, "Loading Kubernetes config")}
	}

	return APIConfigOrError{Config: &config}
}

func ProvideClusterName(configOrError APIConfigOrError) ClusterName {
	config := configOrError.Config
	if config == nil {
		return ""
	}
	n := config.CurrentContext
	c, ok := config.Contexts[n]
	if !ok {
		return ""
	}
	return ClusterName(c.Cluster)
}

const ProductNone = clusterid.Product("")

func ProvideClusterProduct(configOrError APIConfigOrError) clusterid.Product {
	config := configOrError.Config
	if config == nil {
		return ProductNone
	}
	return ClusterProductFromAPIConfig(config)
}

func ClusterProductFromAPIConfig(config *api.Config) clusterid.Product {
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
