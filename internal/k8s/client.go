package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
)

type Client interface {
	Apply(ctx context.Context, rawYAML string) error
}

func DefaultClient() Client {
	return NewKubectlClient()
}

type kubectlClient struct {
}

func NewKubectlClient() kubectlClient {
	return kubectlClient{}
}

func (k kubectlClient) Apply(ctx context.Context, rawYAML string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sApply")
	defer span.Finish()
	// TODO(dmiller) validate that the string is YAML and give a good error
	logger.Get(ctx).Infof("%sapplying via kubectl", logger.Tab)
	c := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	writer := output.Get(ctx).Writer()

	c.Stdout = writer

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, writer)

	err := c.Run()
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}
