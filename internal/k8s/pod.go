package k8s

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/container"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
	return podAPI.Watch(watchOptions)
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
	return req.Stream()
}

func (k K8sClient) PodByID(ctx context.Context, pID PodID, n Namespace) (*v1.Pod, error) {
	return k.core.Pods(n.String()).Get(pID.String(), metav1.GetOptions{})
}

func (k K8sClient) PollForPodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []model.LabelPair, timeout time.Duration) ([]v1.Pod, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PollForPodsWithImage")
	span.SetTag("img", image.String())
	defer span.Finish()

	start := time.Now()
	for time.Since(start) < timeout {
		pod, err := k.PodsWithImage(ctx, image, n, labels)
		if err != nil {
			return nil, err
		}

		if pod != nil {
			return pod, nil
		}
	}

	return nil, fmt.Errorf("timed out polling for pod running image %s (after %s)",
		image.String(), timeout)
}

// PodsWithImage returns the ID of the pod running the given image. If too many matches, throw
// an error. If no matches, return nil -- nothing is wrong, we just didn't find a result.
func (k K8sClient) PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []model.LabelPair) ([]v1.Pod, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PodsWithImage")
	defer span.Finish()

	podList, err := k.core.Pods(n.String()).List(metav1.ListOptions{
		LabelSelector: makeLabelSelector(labels),
	})
	if err != nil {
		return nil, errors.Wrap(err, "PodsWithImage")
	}

	ip := podMap(podList)
	pods, ok := ip[image.String()]
	if !ok {
		// Nothing's wrong, we just didn't find a match.
		return nil, nil
	}
	return pods, nil
}

func podMap(podList *v1.PodList) map[string][]v1.Pod {
	ip := make(map[string][]v1.Pod, 0)
	for _, p := range podList.Items {
		for _, c := range p.Spec.Containers {
			imgRef := c.Image

			// normalize the image name
			ref, err := reference.ParseNormalizedNamed(imgRef)
			if err == nil {
				imgRef = ref.String()
			}

			ip[imgRef] = append(ip[imgRef], p)
		}
	}
	return ip
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
