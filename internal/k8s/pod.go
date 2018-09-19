package k8s

import (
	"bufio"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
)

func (k K8sClient) PollForPodWithImage(ctx context.Context, image reference.NamedTagged, timeout time.Duration) (PodID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PollForPodWithImage")
	span.SetTag("img", image.String())
	defer span.Finish()

	start := time.Now()
	for time.Since(start) < timeout {
		pID, err := k.PodWithImage(ctx, image)
		if err != nil {
			return "", err
		}
		if !pID.Empty() {
			return pID, nil
		}
	}

	return "", fmt.Errorf("timed out polling for pod running image %s (after %s)",
		image.String(), timeout)
}

// PodWithImage returns the ID of the pod running the given image. If too many matches, throw
// an error. If no matches, return nil -- nothing is wrong, we just didn't find a result.
func (k K8sClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8sClient-PodWithImage")
	defer span.Finish()

	ip, err := k.imagesToPods(ctx)
	if err != nil {
		return "", err
	}
	pods, ok := ip[image.String()]
	if !ok {
		// Nothing's wrong, we just didn't find a match.
		return "", nil
	}
	if len(pods) > 1 {
		return "", fmt.Errorf("too many pods found for %s: %d", image, len(pods))
	}
	return pods[0], nil
}

func (k K8sClient) imagesToPods(ctx context.Context) (map[string][]PodID, error) {
	stdout, stderr, err := k.kubectlRunner.exec(ctx, []string{"get", "pods", `-o=jsonpath='{range .items[*]}{"\n"}{.metadata.name}{"\t"}{range .spec.containers[*]}{.image}{"\t"}'`})

	if err != nil {
		return nil, fmt.Errorf("imagesToPods %v (with stderr: %s)", err, stderr)

	}
	return imgPodMapFromOutput(stdout)
}

func imgPodMapFromOutput(output string) (map[string][]PodID, error) {
	imgsToPods := make(map[string][]PodID)
	lns := strings.Split(output, "\n")
	for _, ln := range lns {
		if strings.TrimSpace(ln) == "" {
			continue
		}

		tuple := strings.Split(ln, "\t")
		if len(tuple) == 0 {
			return nil, fmt.Errorf("could not split line on tab: %s", ln)
		}

		for _, nt := range tuple[1:] {
			imgsToPods[nt] = append(imgsToPods[nt], PodID(tuple[0]))
		}
	}
	return imgsToPods, nil
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
