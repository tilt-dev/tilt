package k8s

import (
	"context"
	"net/http"
	"sync"

	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/logger"
)

type runtimeAsync struct {
	core    apiv1.CoreV1Interface
	runtime container.Runtime
	once    sync.Once
}

func newRuntimeAsync(core apiv1.CoreV1Interface) *runtimeAsync {
	return &runtimeAsync{
		core:    core,
		runtime: container.RuntimeUnknown,
	}
}

func (r *runtimeAsync) Runtime(ctx context.Context) container.Runtime {
	r.once.Do(func() {
		nodeList, err := r.core.Nodes().List(metav1.ListOptions{
			Limit: 1,
		})
		if err != nil {
			logger.Get(ctx).Debugf("Error fetching nodes: %v", err)

			statusErr, isStatusErr := err.(*apiErrors.StatusError)
			if isStatusErr {
				status := statusErr.ErrStatus
				if status.Code == http.StatusForbidden {
					logger.Get(ctx).Warnf(
						"Tilt could not read your node configuration\n"+
							"  Ask your Kubernetes admin for access to run `kubectl get nodes`.\n"+
							"  Detail: %v", err)
				}
			}
		}
		if nodeList == nil || len(nodeList.Items) == 0 {
			r.runtime = container.RuntimeReadFailure
			return
		}

		node := nodeList.Items[0]
		info := node.Status.NodeInfo
		r.runtime = container.RuntimeFromVersionString(info.ContainerRuntimeVersion)
	})
	return r.runtime
}

func (c K8sClient) ContainerRuntime(ctx context.Context) container.Runtime {
	return c.runtimeAsync.Runtime(ctx)
}

func ProvideContainerRuntime(ctx context.Context, kCli Client) container.Runtime {
	return kCli.ContainerRuntime(ctx)
}
