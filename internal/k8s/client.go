package k8s

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"
	"github.com/windmilleng/tilt/internal/logger"
)

type PodID string
type ContainerID string
type NodeID string

func (pID PodID) String() string { return string(pID) }

func (cID ContainerID) String() string { return string(cID) }
func (cID ContainerID) ShortStr() string {
	if len(string(cID)) > 10 {
		return string(cID)[:10]
	}
	return string(cID)
}

func (nID NodeID) String() string { return string(nID) }

type Client interface {
	Apply(ctx context.Context, entities []K8sEntity) error
	Delete(ctx context.Context, entities []K8sEntity) error

	PodWithImage(ctx context.Context, image reference.NamedTagged) (PodID, error)

	// Gets the ID for the Node on which the specified Pod is running
	GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error)

	// Finds the PodID for the instance of appName running on the same node as podID
	FindAppByNode(ctx context.Context, appName string, nodeID NodeID) (PodID, error)

	// Waits for the LoadBalancer to get a publicly available URL,
	// then opens that URL in a web browser.
	OpenService(ctx context.Context, lb LoadBalancer) error
}

type KubectlClient struct {
	env           Env
	kubectlRunner kubectlRunner
}

var _ Client = KubectlClient{}

func NewKubectlClient(ctx context.Context, env Env) KubectlClient {
	// TODO(nick): I'm not happy about the way that pkg/browser uses global writers.
	writer := logger.Get(ctx).Writer(logger.DebugLvl)
	browser.Stdout = writer
	browser.Stderr = writer

	return KubectlClient{
		env:           env,
		kubectlRunner: realKubectlRunner{},
	}
}

func (k KubectlClient) OpenService(ctx context.Context, lb LoadBalancer) error {
	if k.env == EnvDockerDesktop && len(lb.Ports) > 0 {
		url := fmt.Sprintf("http://localhost:%d/", lb.Ports[0])
		logger.Get(ctx).Infof("Opening browser: %s", url)
		return browser.OpenURL(url)
	}

	if k.env == EnvMinikube {
		logger.Get(ctx).Infof("Waiting on minikube to load service: %s", lb.Name)

		intervalSec := "1" // 1s is the smallest polling interval we can set :raised_eyebrow:
		cmd := exec.CommandContext(ctx, "minikube", "service", lb.Name, "--url", "--interval", intervalSec)

		cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

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
	logger.Get(ctx).Infof("%sApplying via kubectl", logger.Tab)
	_, stderr, err := k.kubectlRunner.cli(ctx, "apply", entities...)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderr)
	}
	return nil
}

func (k KubectlClient) Delete(ctx context.Context, entities []K8sEntity) error {
	_, _, err := k.kubectlRunner.cli(ctx, "delete", entities...)
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
