package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
)

func Apply(ctx context.Context, rawYAML string, stdout io.Writer, stderr io.Writer) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sApply")
	defer span.Finish()
	// TODO(dmiller) validate that the string is YAML and give a good error
	logger.Get(ctx).Verbose("- Applying YAML via kubectl")
	c := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	c.Stdout = stdout

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, stderr)

	err := c.Run()
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}
