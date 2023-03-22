package k8s

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/localregistry-go"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Recommended in Tilt-specific scripts
const tiltAnnotationRegistry = "tilt.dev/registry"
const tiltAnnotationRegistryFromCluster = "tilt.dev/registry-from-cluster"

// Recommended in Kind's scripts
// https://kind.sigs.k8s.io/docs/user/local-registry/
// There's active work underway to standardize this.
const kindAnnotationRegistry = "kind.x-k8s.io/registry"

const microk8sRegistryNamespace = "container-registry"
const microk8sRegistryName = "registry"

type RuntimeSource interface {
	Runtime(ctx context.Context) container.Runtime
}

type NaiveRuntimeSource struct {
	runtime container.Runtime
}

func NewNaiveRuntimeSource(r container.Runtime) NaiveRuntimeSource {
	return NaiveRuntimeSource{runtime: r}
}

func (s NaiveRuntimeSource) Runtime(ctx context.Context) container.Runtime {
	return s.runtime
}

type registryAsync struct {
	env           clusterid.Product
	core          apiv1.CoreV1Interface
	runtimeSource RuntimeSource
	registry      *v1alpha1.RegistryHosting
	once          sync.Once
}

func newRegistryAsync(env clusterid.Product, core apiv1.CoreV1Interface, runtimeSource RuntimeSource) *registryAsync {
	return &registryAsync{
		env:           env,
		core:          core,
		runtimeSource: runtimeSource,
	}
}

func (r *registryAsync) inferRegistryFromMicrok8s(ctx context.Context) *v1alpha1.RegistryHosting {
	// If Microk8s is using the docker runtime, we can just use the microk8s docker daemon
	// instead of the registry.
	runtime := r.runtimeSource.Runtime(ctx)
	if runtime == container.RuntimeDocker {
		return nil
	}

	// Microk8s might have a registry enabled.
	// https://microk8s.io/docs/working
	svc, err := r.core.Services(microk8sRegistryNamespace).Get(ctx, microk8sRegistryName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Get(ctx).Warnf("You are running microk8s without a local image registry.\n" +
				"Run: `sudo microk8s.enable registry`\n" +
				"Tilt will use the local registry to speed up builds")
		} else {
			logger.Get(ctx).Debugf("Error fetching services: %v", err)
		}
		return nil
	}

	portSpecs := svc.Spec.Ports
	if len(portSpecs) == 0 {
		return nil
	}

	// Check to make sure localhost resolves to an IPv4 address. If it doesn't,
	// then we won't be able to connect to the registry. See:
	// https://github.com/tilt-dev/tilt/issues/2369
	ips, err := net.LookupIP("localhost")
	if err != nil || len(ips) == 0 || ips[0].To4() == nil {
		logger.Get(ctx).Warnf("Your /etc/hosts is resolving localhost to ::1 (IPv6).\n" +
			"This breaks the microk8s image registry.\n" +
			"Please fix your /etc/hosts to default to IPv4. This will make image pushes much faster.")
		return nil
	}

	portSpec := portSpecs[0]
	host := fmt.Sprintf("localhost:%d", portSpec.NodePort)
	reg := v1alpha1.RegistryHosting{Host: host}
	if err := reg.Validate(ctx); err != nil {
		logger.Get(ctx).Warnf("Error validating private registry host %q: %v", host, err.ToAggregate())
		return nil
	}

	return &reg
}

// If this node has the Tilt registry annotations on it, then we can
// infer it was set up with a Tilt script and thus has a local registry.
func (r *registryAsync) inferRegistryFromNodeAnnotations(ctx context.Context) *v1alpha1.RegistryHosting {
	nodeList, err := r.core.Nodes().List(ctx, metav1.ListOptions{Limit: 1})
	if err != nil || len(nodeList.Items) == 0 {
		return nil
	}

	node := nodeList.Items[0]
	annotations := node.Annotations

	var reg *v1alpha1.RegistryHosting
	if tiltReg := annotations[tiltAnnotationRegistry]; tiltReg != "" {
		reg = &v1alpha1.RegistryHosting{
			Host:                     annotations[tiltAnnotationRegistry],
			HostFromContainerRuntime: annotations[tiltAnnotationRegistryFromCluster],
		}
	} else if kindReg := annotations[kindAnnotationRegistry]; kindReg != "" {
		reg = &v1alpha1.RegistryHosting{Host: kindReg}
	}

	if reg != nil {
		if err := reg.Validate(ctx); err != nil {
			logger.Get(ctx).Warnf("Local registry read from node failed: %v", err.ToAggregate())
			return nil
		}
	}

	return reg
}

// Implements the local registry discovery standard.
func (r *registryAsync) inferRegistryFromConfigMap(ctx context.Context) (registry *v1alpha1.RegistryHosting, help string) {
	hosting, err := localregistry.Discover(ctx, r.core)
	if err != nil {
		logger.Get(ctx).Debugf("Local registry discovery error: %v", err)
		return nil, ""
	}

	if hosting.Host == "" {
		return nil, hosting.Help
	}

	registry = &v1alpha1.RegistryHosting{
		Host:                     hosting.Host,
		HostFromClusterNetwork:   hosting.HostFromClusterNetwork,
		HostFromContainerRuntime: hosting.HostFromContainerRuntime,
		Help:                     hosting.Help,
	}

	if r.env == clusterid.ProductUnknown && strings.Contains(registry.Host, "/") {
		names := strings.SplitN(registry.Host, "/", 2)
		registry.Host = names[0]
		registry.SingleName = names[1]
	}

	if err := registry.Validate(ctx); err != nil {
		logger.Get(ctx).Debugf("Local registry discovery error: %v", err.ToAggregate())
		return nil, hosting.Help
	}
	return registry, hosting.Help
}

func (r *registryAsync) Registry(ctx context.Context) *v1alpha1.RegistryHosting {
	r.once.Do(func() {
		reg, help := r.inferRegistryFromConfigMap(ctx)
		if !container.IsEmptyRegistry(reg) {
			r.registry = reg
			return
		}

		// Auto-infer the microk8s local registry.
		if r.env == clusterid.ProductMicroK8s {
			reg := r.inferRegistryFromMicrok8s(ctx)
			if !container.IsEmptyRegistry(reg) {
				r.registry = reg
				return
			}
		}

		reg = r.inferRegistryFromNodeAnnotations(ctx)
		if !container.IsEmptyRegistry(reg) {
			r.registry = reg
		}

		if container.IsEmptyRegistry(r.registry) {
			if help != "" {
				logger.Get(ctx).Warnf("You are running without a local image registry.\n"+
					"Tilt can use the local registry to speed up builds.\n"+
					"Instructions: %s", help)
			} else if r.env == clusterid.ProductKIND {
				logger.Get(ctx).Warnf("You are running Kind without a local image registry.\n" +
					"Tilt can use the local registry to speed up builds.\n" +
					"Instructions: https://github.com/tilt-dev/kind-local")
			}
		}
	})
	return r.registry
}

func (c K8sClient) LocalRegistry(ctx context.Context) *v1alpha1.RegistryHosting {
	return c.registryAsync.Registry(ctx)
}
