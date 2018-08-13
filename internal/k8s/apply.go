package k8s

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"io"
)

func Apply(ctx context.Context, rawYAML string, stdout io.Writer, stderr io.Writer) error {
	// TODO(dmiller) validate that the string is YAML and give a good error
	//c := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	c := exec.CommandContext(ctx, "bash", "-c", "echo hi")
	//r := bytes.NewReader([]byte(rawYAML))
	//c.Stdin = r

	c.Stdout = stdout

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, stderr)

	err := c.Run()
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}
