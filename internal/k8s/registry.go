package k8s

import (
	"context"
	"fmt"
	"sync"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/logger"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

const microk8sRegistryNamespace = "container-registry"
const microk8sRegistryName = "registry"

type registryAsync struct {
	env      Env
	core     apiv1.CoreV1Interface
	registry container.Registry
	once     sync.Once
}

func newRegistryAsync(env Env, core apiv1.CoreV1Interface) *registryAsync {
	return &registryAsync{
		env:  env,
		core: core,
	}
}

func (r *registryAsync) Registry(ctx context.Context) container.Registry {
	r.once.Do(func() {
		if r.env != EnvMicroK8s {
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
