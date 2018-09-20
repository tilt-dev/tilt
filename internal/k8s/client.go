package k8s

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"
	"github.com/windmilleng/tilt/internal/logger"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const PauseCmd = "/pause"

type PodID string
type ContainerID string
type NodeID string

func (pID PodID) String() string { return string(pID) }
func (pID PodID) Empty() bool    { return pID.String() == "" }

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
	PollForPodWithImage(ctx context.Context, image reference.NamedTagged, timeout time.Duration) (PodID, error)

	// Gets the ID for the Node on which the specified Pod is running
	GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error)

	// Finds the PodID for the instance of appName running on the same node as podID
	FindAppByNode(ctx context.Context, nodeID NodeID, appName string, options FindAppByNodeOptions) (PodID, error)

	// Waits for the LoadBalancerSpec to get a publicly available URL.
	ResolveLoadBalancer(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error)

	// Opens a tunnel to the specified pod+port. Returns the tunnel's local port and a function that closes the tunnel
	ForwardPort(ctx context.Context, namespace string, podID PodID, remotePort int) (localPort int, closer func(), err error)
}

type K8sClient struct {
	env           Env
	kubectlRunner kubectlRunner
	core          v1.CoreV1Interface
	restConfig    *rest.Config
	portForwarder PortForwarder
}

var _ Client = K8sClient{}

type PortForwarder func(ctx context.Context, restConfig *rest.Config, core v1.CoreV1Interface, namespace string, podID PodID, localPort int, remotePort int) (closer func(), err error)

func NewK8sClient(
	ctx context.Context,
	env Env,
	core v1.CoreV1Interface,
	restConfig *rest.Config,
	pf PortForwarder) K8sClient {

	// TODO(nick): I'm not happy about the way that pkg/browser uses global writers.
	writer := logger.Get(ctx).Writer(logger.DebugLvl)
	browser.Stdout = writer
	browser.Stderr = writer

	return K8sClient{
		env:           env,
		kubectlRunner: realKubectlRunner{},
		core:          core,
		restConfig:    restConfig,
		portForwarder: pf,
	}
}

func (k K8sClient) ResolveLoadBalancer(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error) {
	if k.env == EnvDockerDesktop && len(lb.Ports) > 0 {
		url, err := url.Parse(fmt.Sprintf("http://localhost:%d/", lb.Ports[0]))
		if err != nil {
			return LoadBalancer{}, fmt.Errorf("hard-coded url failed to parse??? : %v", err)
		}
		return LoadBalancer{
			URL:  url,
			Spec: lb,
		}, nil
	}

	if k.env == EnvMinikube {
		return k.resolveLoadBalancerFromMinikube(ctx, lb)
	}

	return k.resolveLoadBalancerFromK8sAPI(ctx, lb)
}

func (k K8sClient) resolveLoadBalancerFromMinikube(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error) {
	logger.Get(ctx).Infof("Waiting on minikube to resolve service: %s", lb.Name)

	intervalSec := "1" // 1s is the smallest polling interval we can set :raised_eyebrow:
	cmd := exec.CommandContext(ctx, "minikube", "service", lb.Name, "--url", "--interval", intervalSec)

	cmd.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

	out, err := cmd.Output()
	if err != nil {
		return LoadBalancer{}, fmt.Errorf("ResolveLoadBalancer: %v", err)
	}
	url, err := url.Parse(strings.TrimSpace(string(out)))
	if err != nil {
		return LoadBalancer{}, fmt.Errorf("ResolveLoadBalancer: malformed url: %v", err)
	}
	return LoadBalancer{
		URL:  url,
		Spec: lb,
	}, nil
}

func (k K8sClient) resolveLoadBalancerFromK8sAPI(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error) {
	if len(lb.Ports) == 0 {
		return LoadBalancer{}, nil
	}

	port := lb.Ports[0]

	// TODO(nick): Use lb.Namespace when it's committed.
	svc, err := k.core.Services("default").Get(lb.Name, metav1.GetOptions{})
	if err != nil {
		return LoadBalancer{}, fmt.Errorf("ResolveLoadBalancer#Services: %v", err)
	}

	status := svc.Status
	lbStatus := status.LoadBalancer

	// Documentation here is helpful:
	// https://godoc.org/k8s.io/api/core/v1#LoadBalancerIngress
	// GKE and OpenStack typically use IP-based load balancers.
	// AWS typically uses DNS-based load balancers.
	for _, ingress := range lbStatus.Ingress {
		urlString := ""
		if ingress.IP != "" {
			urlString = fmt.Sprintf("http://%s:%d/", ingress.IP, port)
		}

		if ingress.Hostname != "" {
			urlString = fmt.Sprintf("http://%s:%d/", ingress.Hostname, port)
		}

		if urlString == "" {
			continue
		}

		url, err := url.Parse(urlString)
		if err != nil {
			return LoadBalancer{}, fmt.Errorf("ResolveLoadBalancer: malformed url: %v", err)
		}
		return LoadBalancer{
			URL:  url,
			Spec: lb,
		}, nil
	}

	return LoadBalancer{}, nil
}

func (k K8sClient) Apply(ctx context.Context, entities []K8sEntity) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sApply")
	defer span.Finish()
	// TODO(dmiller) validate that the string is YAML and give a good error
	logger.Get(ctx).Infof("%sApplying via kubectl", logger.Tab)
	_, stderr, err := k.applyOrDeleteFromEntities(ctx, "apply", entities)
	if err != nil {
		return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderr)
	}
	return nil
}

func (k K8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	_, _, err := k.applyOrDeleteFromEntities(ctx, "delete", entities)
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

func (k K8sClient) applyOrDeleteFromEntities(ctx context.Context, cmd string, entities []K8sEntity) (stdout string, stderr string, err error) {
	args := []string{cmd, "-f", "-"}

	rawYAML, err := SerializeYAML(entities)
	if err != nil {
		return "", "", fmt.Errorf("serializeYaml for kubectl %s: %v", cmd, err)
	}
	stdin := bytes.NewReader([]byte(rawYAML))

	return k.kubectlRunner.execWithStdin(ctx, args, stdin)
}

func ProvideCoreInterface(cfg *rest.Config) (v1.CoreV1Interface, error) {
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientSet.CoreV1(), nil
}

func ProvideRESTClient(cfg *rest.Config) (v1.CoreV1Interface, error) {
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientSet.CoreV1(), nil
}

func ProvideRESTConfig() (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{}

	clientLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		overrides)
	config, err := clientLoader.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf(
			"could not get config for context (%q): %s", overrides.Context, err)
	}

	return config, nil
}

func ProvidePortForwarder() PortForwarder {
	return portForwarder
}

func OpenService(ctx context.Context, client Client, lbSpec LoadBalancerSpec) error {
	lb, err := client.ResolveLoadBalancer(ctx, lbSpec)
	if err != nil {
		return err
	}

	if lb.URL == nil {
		logger.Get(ctx).Infof("Could not determine URL of service: %s", lbSpec.Name)
		return nil
	}

	return browser.OpenURL(lb.URL.String())
}
