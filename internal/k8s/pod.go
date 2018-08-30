package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"
)

// PodWithImage returns the ID of the pod running the given image. We expect exactly one
// matching pod: if too many or too few matches, throw an error.
func (k KubectlClient) PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error) {
	_, err := imagesToPods(ctx)
	if err != nil {
		return PodID(""), err
	}
	// pods := imgPodMap[image.String()]
	return PodID(""), nil
}

func imagesToPods(ctx context.Context) (map[reference.NamedTagged][]PodID, error) {
	c := exec.CommandContext(ctx, "kubectl", "get", "pods", "-o=jsonpath={range .items[*]}{\"\\n\"}{.metadata.name}{\"\\t\"}{range .spec.containers[*]}{.image}")
	stdoutBuf, stderrBuf := &bytes.Buffer{}, &bytes.Buffer{}
	c.Stdout = stdoutBuf
	c.Stderr = stderrBuf

	err := c.Run()
	if err != nil {
		return nil, fmt.Errorf("imagesToPods: %v (stderr: %s)", err, stderrBuf.String())
	}

	fmt.Printf(stdoutBuf.String())
	return imgPodMapFromOutput(stdoutBuf.String())
}

func imgPodMapFromOutput(output string) (map[reference.NamedTagged][]PodID, error) {
	imgsToPods := make(map[reference.NamedTagged][]PodID)
	lns := strings.Split(output, "\n")
	for _, ln := range lns {
		if strings.TrimSpace(ln) == "" {
			continue
		}

		pair := strings.Split(ln, "\t")
		if len(pair) != 2 {
			return nil, fmt.Errorf("could not split line in two on tab: %s", ln)
		}

		nt, err := ParseNamedTagged(pair[1])
		if err != nil {
			return nil, err
		}

		imgsToPods[nt] = append(imgsToPods[nt], PodID(pair[0]))
	}
	return imgsToPods, nil
}
