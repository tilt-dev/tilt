package k8s

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"
	opentracing "github.com/opentracing/opentracing-go"
)

// PodWithImage returns the ID of the pod running the given image. We expect exactly one
// matching pod: if too many or too few matches, throw an error.
func (k KubectlClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "kubectlClient-PodWithImage")
	defer span.Finish()

	ip, err := imagesToPods(ctx)
	if err != nil {
		return PodID(""), err
	}
	pods, ok := ip[image.String()]
	if !ok {
		return PodID(""), fmt.Errorf("unable to find pods for %s. Found: %+v", image, ip)
	}
	if len(pods) > 1 {
		return PodID(""), fmt.Errorf("too many pods found for %s: %d", image, len(pods))
	}
	return pods[0], nil
}

func imagesToPods(ctx context.Context) (map[string][]PodID, error) {
	c := exec.CommandContext(ctx, "kubectl", "get", "pods", `-o=jsonpath={range .items[*]}{"\n"}{.metadata.name}{"\t"}{range .spec.containers[*]}{.image}{"\t"}`)

	out, err := c.Output()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("imagesToPods: stderr: %s)", exitError.Stderr)
		} else {
			return nil, fmt.Errorf("imagesToPods: %v", err.Error())
		}
	}

	return imgPodMapFromOutput(string(out))
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
