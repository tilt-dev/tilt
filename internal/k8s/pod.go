package k8s

import (
	"context"
	"io"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/container"
)

func (k *K8sClient) ContainerLogs(ctx context.Context, pID PodID, cName container.Name, n Namespace, startWatchTime time.Time) (io.ReadCloser, error) {
	options := &v1.PodLogOptions{
		Container: cName.String(),
		Follow:    true,
		SinceTime: &metav1.Time{
			Time: startWatchTime,
		},
	}
	req := k.core.Pods(n.String()).GetLogs(pID.String(), options)
	return req.Stream(ctx)
}

func PodIDFromPod(pod *v1.Pod) PodID {
	return PodID(pod.ObjectMeta.Name)
}

func NamespaceFromPod(pod *v1.Pod) Namespace {
	return Namespace(pod.ObjectMeta.Namespace)
}

func NodeIDFromPod(pod *v1.Pod) NodeID {
	return NodeID(pod.Spec.NodeName)
}
