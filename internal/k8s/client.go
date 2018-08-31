package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/output"
)

type PodID string
type ContainerID string

func (cID ContainerID) String() string { return string(cID) }
func (cID ContainerID) ShortStr() string {
	if len(string(cID)) > 10 {
		return string(cID)[:10]
	}
	return string(cID)
}

type Client interface {
	Apply(ctx context.Context, entities []K8sEntity) error
	Delete(ctx context.Context, entities []K8sEntity) error

	PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error)

	// Waits for the LoadBalancer to get a publicly available URL,
	// then opens that URL in a web browser.
	OpenService(ctx context.Context, lb LoadBalancer) error
}

type KubectlClient struct {
	env Env
}

func NewKubectlClient(ctx context.Context, env Env) KubectlClient {
	// TODO(nick): I'm not happy about the way that pkg/browser uses global writers.
	writer := logger.Get(ctx).Writer(logger.DebugLvl)
	browser.Stdout = writer
	browser.Stderr = writer

	return KubectlClient{
		env: env,
	}
}

func (k KubectlClient) OpenService(ctx context.Context, lb LoadBalancer) error {
	if k.env == EnvDockerDesktop && len(lb.Ports) > 0 {
		url := fmt.Sprintf("http://localhost:%d/", lb.Ports[0])
		logger.Get(ctx).Infof("Opening browser: %s", url)
		return browser.OpenURL(url)
	}

	if k.env == EnvMinikube {
		cmd := exec.CommandContext(ctx, "minikube", "service", lb.Name, "--url")
		out, err := cmd.Output()
		if err != nil {
			return fmt.Errorf("OpenService: %v", err)
		}
		url, err := url.Parse(strings.TrimSpace(string(out)))
		if err != nil {
			return fmt.Errorf("OpenService: malformed url: %v", err)
		}
		logger.Get(ctx).Infof("Opening browser: %s", url)
		return browser.OpenURL(url.String())
	}

	logger.Get(ctx).Infof("Could not determine URL of service: %s", lb.Name)
	return nil
}

func (k KubectlClient) Apply(ctx context.Context, entities []K8sEntity) error {
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

func (k KubectlClient) Delete(ctx context.Context, entities []K8sEntity) error {
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

func (k KubectlClient) cli(ctx context.Context, cmd string, entities []K8sEntity) (*bytes.Buffer, error) {
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
