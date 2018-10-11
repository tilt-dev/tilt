package k8s

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"
	"github.com/windmilleng/tilt/internal/logger"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Namespace string
type PodID string
type ContainerID string
type ContainerName string
type NodeID string

const DefaultNamespace = Namespace("default")

func (pID PodID) Empty() bool    { return pID.String() == "" }
func (pID PodID) String() string { return string(pID) }

func (cID ContainerID) Empty() bool    { return cID.String() == "" }
func (cID ContainerID) String() string { return string(cID) }
func (cID ContainerID) ShortStr() string {
	if len(string(cID)) > 10 {
		return string(cID)[:10]
	}
	return string(cID)
}

func (n ContainerName) String() string { return string(n) }

func (nID NodeID) String() string { return string(nID) }

func (n Namespace) String() string {
	if n == "" {
		return string(DefaultNamespace)
	}
	return string(n)
}

type Client interface {
	// Updates the entities, creating them if necessary.
	//
	// Tries to update them in-place if possible. But for certain resource types,
	// we might need to fallback to deleting and re-creating them.
	Upsert(ctx context.Context, entities []K8sEntity) error

	// Find all the pods that match the given image, namespace, and labels.
	PodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []LabelPair) ([]v1.Pod, error)

	// Find all the pods matching the given parameters, stopping on timeout or
	// when we have at least one pod.
	PollForPodsWithImage(ctx context.Context, image reference.NamedTagged, n Namespace, labels []LabelPair, timeout time.Duration) ([]v1.Pod, error)

	PodByID(ctx context.Context, podID PodID, n Namespace) (*v1.Pod, error)

	// Creates a channel where all changes to the pod are brodcast.
	// Takes a pod as input, to indicate the version of the pod where we start watching.
	WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error)

	// Streams the container logs
	ContainerLogs(ctx context.Context, podID PodID, cName ContainerName, n Namespace) (io.ReadCloser, error)

	// Gets the ID for the Node on which the specified Pod is running
	GetNodeForPod(ctx context.Context, podID PodID) (NodeID, error)

	// Finds the PodID for the instance of appName running on the same node as podID
	FindAppByNode(ctx context.Context, nodeID NodeID, appName string, options FindAppByNodeOptions) (PodID, error)

	// Waits for the LoadBalancerSpec to get a publicly available URL.
	ResolveLoadBalancer(ctx context.Context, lb LoadBalancerSpec) (LoadBalancer, error)

	// Opens a tunnel to the specified pod+port. Returns the tunnel's local port and a function that closes the tunnel
	ForwardPort(ctx context.Context, namespace Namespace, podID PodID, remotePort int) (localPort int, closer func(), err error)

	WatchPods(ctx context.Context, lps []LabelPair) (<-chan *v1.Pod, error)
}

type K8sClient struct {
	env           Env
	kubectlRunner kubectlRunner
	core          apiv1.CoreV1Interface
	restConfig    *rest.Config
	portForwarder PortForwarder
}

var _ Client = K8sClient{}

type PortForwarder func(ctx context.Context, restConfig *rest.Config, core apiv1.CoreV1Interface, namespace string, podID PodID, localPort int, remotePort int) (closer func(), err error)

func NewK8sClient(
	ctx context.Context,
	env Env,
	core apiv1.CoreV1Interface,
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

	svc, err := k.core.Services(lb.Namespace.String()).Get(lb.Name, metav1.GetOptions{})
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

func (k K8sClient) Upsert(ctx context.Context, entities []K8sEntity) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sUpsert")
	defer span.Finish()

	logger.Get(ctx).Infof("%sApplying via kubectl", logger.Tab)

	immutable := ImmutableEntities(entities)
	if len(immutable) > 0 {
		_, stderr, err := k.actOnEntities(ctx, []string{"replace", "--force"}, immutable)
		if err != nil {
			return fmt.Errorf("kubectl replace: %v\nstderr: %s", err, stderr)
		}
	}

	mutable := MutableEntities(entities)
	if len(mutable) > 0 {
		_, stderr, err := k.actOnEntities(ctx, []string{"apply"}, mutable)
		if err != nil {
			isImmutableFieldError := strings.Contains(stderr, validation.FieldImmutableErrorMsg)
			if !isImmutableFieldError {
				return fmt.Errorf("kubectl apply: %v\nstderr: %s", err, stderr)
			}

			// If the kubectl apply failed due to an immutable field, fall back to kubectl replace --force.
			logger.Get(ctx).Infof("%sFalling back to 'kubectl replace' on immutable field error", logger.Tab)
			_, stderr, err := k.actOnEntities(ctx, []string{"replace", "--force"}, mutable)
			if err != nil {
				return fmt.Errorf("kubectl replace: %v\nstderr: %s", err, stderr)
			}
		}
	}
	return nil
}

func (k K8sClient) actOnEntities(ctx context.Context, cmdArgs []string, entities []K8sEntity) (stdout string, stderr string, err error) {
	args := append([]string{}, cmdArgs...)
	args = append(args, "-f", "-")

	rawYAML, err := SerializeYAML(entities)
	if err != nil {
		return "", "", fmt.Errorf("serializeYaml for kubectl %s: %v", cmdArgs, err)
	}
	stdin := bytes.NewReader([]byte(rawYAML))

	return k.kubectlRunner.execWithStdin(ctx, args, stdin)
}

func ProvideCoreInterface(cfg *rest.Config) (apiv1.CoreV1Interface, error) {
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return clientSet.CoreV1(), nil
}

func ProvideRESTClient(cfg *rest.Config) (apiv1.CoreV1Interface, error) {
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
