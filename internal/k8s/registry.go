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

func (r *registryAsync) Registry(ctx context.Context) (container.Registry, error) {
	var err error
	r.once.Do(func() {
		// Right now, we only recognize the microk8s private registry.
		if r.env != EnvMicroK8s {
			return
		}

		// If Microk8s is using the docker runtime, we can just use the microk8s docker daemon
		// instead of the registry.
		runtime := r.runtimeSource.Runtime(ctx)
		if runtime == container.RuntimeDocker {
			return
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
			return
		}

		portSpecs := svc.Spec.Ports
		if len(portSpecs) == 0 {
			return
		}

		// Check to make sure localhost resolves to an IPv4 address. If it doesn't,
		// then we won't be able to connect to the registry. See:
		// https://github.com/windmilleng/tilt/issues/2369
		ips, err := net.LookupIP("localhost")
		if err != nil || len(ips) == 0 || ips[0].To4() == nil {
			logger.Get(ctx).Warnf("Your /etc/hosts is resolving localhost to ::1 (IPv6).\n" +
				"This breaks the microk8s image registry.\n" +
				"Please fix your /etc/hosts to default to IPv4. This will make image pushes much faster.")
			return
		}

		portSpec := portSpecs[0]
		r.registry, err = container.NewRegistry(fmt.Sprintf("localhost:%d", portSpec.NodePort))
	})
	return r.registry, err
}

func (c K8sClient) PrivateRegistry(ctx context.Context) (container.Registry, error) {
	return c.registryAsync.Registry(ctx)
}

func ProvideContainerRegistry(ctx context.Context, kCli Client) (container.Registry, error) {
	return kCli.PrivateRegistry(ctx)
}
