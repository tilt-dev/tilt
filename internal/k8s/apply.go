package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	"github.com/windmilleng/tilt/internal/logger"
)

func Apply(ctx context.Context, rawYAML string) error {
	// TODO(dmiller) validate that the string is YAML and give a good error
	c := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	writer := logger.Get(ctx).Writer(logger.VerboseLvl)

	c.Stdout = writer

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, writer)

	err := c.Run()
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}
