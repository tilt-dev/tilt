package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/tilt-dev/tilt/internal/container"
)

func (k K8sClient) WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error) {
	podAPI := k.core.Pods(NamespaceFromPod(pod).String())
	podID := PodIDFromPod(pod)
	fieldSelector := fmt.Sprintf("metadata.name=%s", podID)
	watchOptions := metav1.ListOptions{
		FieldSelector:   fieldSelector,
		Watch:           true,
		ResourceVersion: pod.ObjectMeta.ResourceVersion,
	}
	return podAPI.Watch(ctx, watchOptions)
}

func (k K8sClient) ContainerLogs(ctx context.Context, pID PodID, cName container.Name, n Namespace, startWatchTime time.Time) (io.ReadCloser, error) {
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

func (k K8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	pod, err := k.core.Pods(n.String()).Get(ctx, pID.String(), metav1.GetOptions{})
	if pod != nil {
		FixContainerStatusImages(pod)
	}
	return pod, err
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
