package k8s

import (
	"context"
	"fmt"
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

func (r *registryAsync) Registry(ctx context.Context) container.Registry {
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
				logger.Get(ctx).Infof("WARNING: You are running microk8s without a local image registry.\n" +
					"Run: `sudo microk8s.enable registry`\n" +
					"Tilt will use the local registry to speed up builds\n")
			} else {
				logger.Get(ctx).Debugf("Error fetching services: %v", err)
			}
			return
		}

		portSpecs := svc.Spec.Ports
		if len(portSpecs) == 0 {
			return
		}

		portSpec := portSpecs[0]
		r.registry = container.Registry(fmt.Sprintf("localhost:%d", portSpec.NodePort))
	})
	return r.registry
}

func (c K8sClient) PrivateRegistry(ctx context.Context) container.Registry {
	return c.registryAsync.Registry(ctx)
}

func ProvideContainerRegistry(ctx context.Context, kCli Client) container.Registry {
	return kCli.PrivateRegistry(ctx)
}
