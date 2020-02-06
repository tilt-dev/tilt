package k8s

import (
	"context"
	"fmt"
	"net"
	"sync"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/logger"
)

const annotationRegistry = "tilt.dev/registry"
const annotationRegistryFromCluster = "tilt.dev/registry-from-cluster"

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
	env           Env
	core          apiv1.CoreV1Interface
	runtimeSource RuntimeSource
	registry      container.Registry
	once          sync.Once
}

func newRegistryAsync(env Env, core apiv1.CoreV1Interface, runtimeSource RuntimeSource) *registryAsync {
	return &registryAsync{
		env:           env,
		core:          core,
		runtimeSource: runtimeSource,
	}
}

func (r *registryAsync) inferRegistryFromMicrok8s(ctx context.Context) container.Registry {
	// If Microk8s is using the docker runtime, we can just use the microk8s docker daemon
	// instead of the registry.
	runtime := r.runtimeSource.Runtime(ctx)
	if runtime == container.RuntimeDocker {
		return container.Registry{}
	}

	// Microk8s might have a registry enabled.
	// https://microk8s.io/docs/working
	svc, err := r.core.Services(microk8sRegistryNamespace).Get(microk8sRegistryName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Get(ctx).Warnf("You are running microk8s without a local image registry.\n" +
				"Run: `sudo microk8s.enable registry`\n" +
				"Tilt will use the local registry to speed up builds")
		} else {
			logger.Get(ctx).Debugf("Error fetching services: %v", err)
		}
		return container.Registry{}
	}

	portSpecs := svc.Spec.Ports
	if len(portSpecs) == 0 {
		return container.Registry{}
	}

	// Check to make sure localhost resolves to an IPv4 address. If it doesn't,
	// then we won't be able to connect to the registry. See:
	// https://github.com/windmilleng/tilt/issues/2369
	ips, err := net.LookupIP("localhost")
	if err != nil || len(ips) == 0 || ips[0].To4() == nil {
		logger.Get(ctx).Warnf("Your /etc/hosts is resolving localhost to ::1 (IPv6).\n" +
			"This breaks the microk8s image registry.\n" +
			"Please fix your /etc/hosts to default to IPv4. This will make image pushes much faster.")
		return container.Registry{}
	}

	portSpec := portSpecs[0]
	host := fmt.Sprintf("localhost:%d", portSpec.NodePort)
	reg, err := container.NewRegistry(host)
	if err != nil {
		logger.Get(ctx).Warnf("Error validating private registry host %q: %v", host, err)
		return container.Registry{}
	}

	return reg
}

// If this node has the Tilt registry annotations on it, then we can
// infer it was set up with a Tilt script and thus has a local registry.
func (r *registryAsync) inferRegistryFromTiltNodeAnnotations(ctx context.Context) container.Registry {
	nodeList, err := r.core.Nodes().List(metav1.ListOptions{Limit: 1})
	if err != nil || len(nodeList.Items) == 0 {
		return container.Registry{}
	}

	node := nodeList.Items[0]
	annotations := node.Annotations

	fromLocal := annotations[annotationRegistry]
	fromCluster := annotations[annotationRegistryFromCluster]

	if fromLocal != "" {
		reg, err := container.NewRegistryWithHostFromCluster(fromLocal, fromCluster)
		if err != nil {
			logger.Get(ctx).Warnf("Local registry read from node failed to parse (%s, %s): %v", fromLocal, fromCluster, err)
			return container.Registry{}
		}
		return reg
	}

	return container.Registry{}
}

func (r *registryAsync) Registry(ctx context.Context) container.Registry {
	r.once.Do(func() {
		// Auto-infer the microk8s local registry.
		if r.env == EnvMicroK8s {
			reg := r.inferRegistryFromMicrok8s(ctx)
			if !reg.Empty() {
				r.registry = reg
				return
			}
		}

		reg := r.inferRegistryFromTiltNodeAnnotations(ctx)
		if !reg.Empty() {
			r.registry = reg
		}
	})
	return r.registry
}

func (c K8sClient) LocalRegistry(ctx context.Context) container.Registry {
	return c.registryAsync.Registry(ctx)
}
