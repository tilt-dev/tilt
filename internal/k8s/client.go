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
	Apply(ctx context.Context, entities []K8sEntity) error
	Delete(ctx context.Context, entities []K8sEntity) error
}

func DefaultClient() Client {
	return NewKubectlClient()
}

type kubectlClient struct {
}

func NewKubectlClient() kubectlClient {
	return kubectlClient{}
}

func (k kubectlClient) Apply(ctx context.Context, entities []K8sEntity) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sApply")
	defer span.Finish()
	// TODO(dmiller) validate that the string is YAML and give a good error
	logger.Get(ctx).Infof("%sapplying via kubectl", logger.Tab)
	stderrBuf, err := k.cli(ctx, "apply", entities)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderrBuf.String())
	}
	return nil
}

func (k kubectlClient) Delete(ctx context.Context, entities []K8sEntity) error {
	_, err := k.cli(ctx, "delete", entities)
	_, isExitErr := err.(*exec.ExitError)
	if isExitErr {
		// In general, an exit error is ok for our purposes.
		// It just means that the job hasn't been created yet.
		return nil
	}

	if err != nil {
		return fmt.Errorf("kubectl delete: %v", err)
	}
	return err
}

func (k kubectlClient) cli(ctx context.Context, cmd string, entities []K8sEntity) (*bytes.Buffer, error) {
	rawYAML, err := SerializeYAML(entities)
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %v", cmd, err)
	}

	c := exec.CommandContext(ctx, "kubectl", cmd, "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	writer := output.Get(ctx).Writer()

	c.Stdout = writer

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, writer)

	return stderrBuf, c.Run()
}
