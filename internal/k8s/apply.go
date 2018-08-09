package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

func Apply(ctx context.Context, rawYAML string) error {
	// TODO(dmiller) validate that the string is YAML and give a good error
	c := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	stderr := &bytes.Buffer{}
	c.Stderr = stderr

	err := c.Run()
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderr.String())
	}
	return nil
}
