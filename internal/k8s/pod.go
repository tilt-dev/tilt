package k8s

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
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
			ip[c.Image] = append(ip[c.Image], p)
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

func (k K8sClient) GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error) {
	jsonPath := "-o=jsonpath={.spec.nodeName}"
	stdout, stderr, err := k.kubectlRunner.exec(ctx, k.kubeContext, []string{"get", "pods", podID.String(), jsonPath})

	if err != nil {
		return NodeID(""), errors.Wrapf(err, "error finding node for pod '%s':\nstderr: '%s'", podID.String(), stderr)
	}

	lines := nonEmptyLines(stdout)

	if len(lines) == 0 {
		return NodeID(""), fmt.Errorf("kubectl output did not contain a node name for pod '%s': '%s'", podID, stdout)
	} else if len(lines) > 1 {
		return NodeID(""), fmt.Errorf("kubectl returned multiple nodes for pod '%s': '%s'", podID, stdout)
	} else {
		return NodeID(lines[0]), nil
	}
}

type FindAppByNodeOptions struct {
	Namespace string
	Owner     string
}

type MultipleAppsFoundError struct {
	filterDesc string
	pods       []string
}

func (m MultipleAppsFoundError) Error() string {
	return fmt.Sprintf("found multiple apps matching %s: '%s'", m.filterDesc, m.pods)
}

func (k K8sClient) FindAppByNode(ctx context.Context, nodeID NodeID, appName string, options FindAppByNodeOptions) (PodID, error) {
	jsonPath := fmt.Sprintf(`-o=jsonpath={range .items[?(@.spec.nodeName=="%s")]}{.metadata.name}{"\n"}`, nodeID)

	filterDesc := fmt.Sprintf("name '%s', node '%s'", appName, nodeID.String())

	labelArg := fmt.Sprintf("-lapp=%s", appName)
	if len(options.Owner) > 0 {
		labelArg += fmt.Sprintf(",owner=%s", options.Owner)
		filterDesc += fmt.Sprintf(", owner '%s'", options.Owner)
	}

	args := append([]string{"get", "pods", labelArg})

	if len(options.Namespace) > 0 {
		args = append(args, fmt.Sprintf("--namespace=%s", options.Namespace))
		filterDesc += fmt.Sprintf(", namespace '%s'", options.Namespace)
	}
	args = append(args, jsonPath)

	stdout, stderr, err := k.kubectlRunner.exec(ctx, k.kubeContext, args)

	if err != nil {
		return PodID(""), errors.Wrapf(err, "error finding app with %s:\nstderr: '%s'", filterDesc, stderr)
	}

	lines := nonEmptyLines(stdout)

	if len(lines) == 0 {
		return PodID(""), fmt.Errorf("unable to find any apps with %s", filterDesc)
	} else if len(lines) > 1 {
		return PodID(""), MultipleAppsFoundError{filterDesc, lines}
	} else {
		return PodID(lines[0]), nil
	}
}

func nonEmptyLines(s string) []string {
	scanner := bufio.NewScanner(strings.NewReader(s))
	scanner.Split(bufio.ScanWords)

	var ret []string

	for scanner.Scan() {
		ret = append(ret, scanner.Text())
	}

	return ret
}
