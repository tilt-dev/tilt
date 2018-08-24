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
	return k.cli(ctx, "apply", entities)
}

func (k kubectlClient) Delete(ctx context.Context, entities []K8sEntity) error {
	return k.cli(ctx, "delete", entities)
}

func (k kubectlClient) cli(ctx context.Context, cmd string, entities []K8sEntity) error {
	rawYAML, err := SerializeYAML(entities)
	if err != nil {
		return fmt.Errorf("kubectl %s: %v", cmd, err)
	}

	c := exec.CommandContext(ctx, "kubectl", cmd, "-f", "-")
	r := bytes.NewReader([]byte(rawYAML))
	c.Stdin = r

	writer := logger.Get(ctx).Writer(logger.InfoLvl)

	c.Stdout = writer

	stderrBuf := &bytes.Buffer{}

	c.Stderr = io.MultiWriter(stderrBuf, writer)

	err = c.Run()
	if err != nil {
		return fmt.Errorf("kubectl %s: %v\nstderr: %s", cmd, err, stderrBuf.String())
	}
	return nil
}
