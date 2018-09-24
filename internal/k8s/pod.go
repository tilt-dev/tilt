package k8s

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (k K8sClient) PollForPodWithImage(ctx context.Context, image reference.NamedTagged, timeout time.Duration) (*v1.Pod, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PollForPodWithImage")
	span.SetTag("img", image.String())
	defer span.Finish()

	start := time.Now()
	for time.Since(start) < timeout {
		pod, err := k.PodWithImage(ctx, image)
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

// PodWithImage returns the ID of the pod running the given image. If too many matches, throw
// an error. If no matches, return nil -- nothing is wrong, we just didn't find a result.
func (k K8sClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (*v1.Pod, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PodWithImage")
	defer span.Finish()

	// TODO(nick): This should take a Namespace, and maybe some label selectors?
	podList, err := k.core.Pods("default").List(metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("PodWithImage: %v", err)
	}

	ip := podMap(podList)
	pods, ok := ip[image.String()]
	if !ok {
		// Nothing's wrong, we just didn't find a match.
		return nil, nil
	}
	if len(pods) > 1 {
		return nil, fmt.Errorf("too many pods found for %s: %d", image, len(pods))
	}

	pod := pods[0]
	return &pod, nil
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

func NodeIDFromPod(pod *v1.Pod) NodeID {
	return NodeID(pod.Spec.NodeName)
}

func (k K8sClient) GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error) {
	jsonPath := "-o=jsonpath={.spec.nodeName}"
	stdout, stderr, err := k.kubectlRunner.exec(ctx, []string{"get", "pods", podID.String(), jsonPath})

	if err != nil {
		return NodeID(""), fmt.Errorf("error finding node for pod '%s': %v, stderr: '%s'", podID.String(), err.Error(), stderr)
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

	stdout, stderr, err := k.kubectlRunner.exec(ctx, args)

	if err != nil {
		return PodID(""), fmt.Errorf("error finding app with %s: %v, stderr: '%s'", filterDesc, err.Error(), stderr)
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
