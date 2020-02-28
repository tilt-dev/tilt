package k8s

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/pkg/logger"
)

type Namespace string
type PodID string
type NodeID string
type ServiceName string
type KubeContext string

const DefaultNamespace = Namespace("default")

var ForbiddenFieldsRe = regexp.MustCompile(`updates to .* are forbidden`)

func (pID PodID) Empty() bool    { return pID.String() == "" }
func (pID PodID) String() string { return string(pID) }

func (nID NodeID) String() string { return string(nID) }

func (n Namespace) Empty() bool { return n == "" }

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
	//
	// Returns entities in the order that they were applied (which may be different
	// than they were passed in) and with UUIDs from the Kube API
	Upsert(ctx context.Context, entities []K8sEntity) ([]K8sEntity, error)

	// Deletes all given entities.
	//
	// Currently ignores any "not found" errors, because that seems like the correct
	// behavior for our use cases.
	Delete(ctx context.Context, entities []K8sEntity) error

	GetByReference(ctx context.Context, ref v1.ObjectReference) (K8sEntity, error)

	PodByID(ctx context.Context, podID PodID, n Namespace) (*v1.Pod, error)

	// Creates a channel where all changes to the pod are brodcast.
	// Takes a pod as input, to indicate the version of the pod where we start watching.
	WatchPod(ctx context.Context, pod *v1.Pod) (watch.Interface, error)

	// Streams the container logs
	ContainerLogs(ctx context.Context, podID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error)

	// Opens a tunnel to the specified pod+port. Returns the tunnel's local port and a function that closes the tunnel
	CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error)

	WatchPods(ctx context.Context, lps labels.Selector) (<-chan ObjectUpdate, error)

	WatchServices(ctx context.Context, lps labels.Selector) (<-chan *v1.Service, error)

	WatchEvents(ctx context.Context) (<-chan *v1.Event, error)

	ConnectedToCluster(ctx context.Context) error

	ContainerRuntime(ctx context.Context) container.Runtime

	// Some clusters support a local image registry that we can push to.
	LocalRegistry(ctx context.Context) container.Registry

	// Some clusters support a node IP where all servers are reachable.
	NodeIP(ctx context.Context) NodeIP

	Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error
}

type K8sClient struct {
	env               Env
	kubectlRunner     kubectlRunner
	core              apiv1.CoreV1Interface
	restConfig        *rest.Config
	portForwardClient PortForwardClient
	configNamespace   Namespace
	clientset         kubernetes.Interface
	dynamic           dynamic.Interface
	runtimeAsync      *runtimeAsync
	registryAsync     *registryAsync
	nodeIPAsync       *nodeIPAsync
	drm               meta.RESTMapper
}

var _ Client = K8sClient{}

func ProvideK8sClient(
	ctx context.Context,
	env Env,
	maybeRESTConfig RESTConfigOrError,
	maybeClientset ClientsetOrError,
	pfClient PortForwardClient,
	configNamespace Namespace,
	runner kubectlRunner,
	clientLoader clientcmd.ClientConfig) Client {
	if env == EnvNone {
		// No k8s, so no need to get any further configs
		return &explodingClient{err: fmt.Errorf("Kubernetes context not set in %s", clientLoader.ConfigAccess().GetLoadingPrecedence())}
	}

	restConfig, err := maybeRESTConfig.Config, maybeRESTConfig.Error
	if err != nil {
		return &explodingClient{err: err}
	}

	clientset, err := maybeClientset.Clientset, maybeClientset.Error
	if err != nil {
		return &explodingClient{err: err}
	}

	core := clientset.CoreV1()
	runtimeAsync := newRuntimeAsync(core)
	registryAsync := newRegistryAsync(env, core, runtimeAsync)
	nodeIPAsync := newNodeIPAsync(env)

	di, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return &explodingClient{err: err}
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return &explodingClient{fmt.Errorf("unable to create discovery client: %v", err)}
	}

	drm := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	// TODO(nick): I'm not happy about the way that pkg/browser uses global writers.
	writer := logger.Get(ctx).Writer(logger.DebugLvl)
	browser.Stdout = writer
	browser.Stderr = writer

	return K8sClient{
		env:               env,
		kubectlRunner:     runner,
		core:              core,
		restConfig:        restConfig,
		portForwardClient: pfClient,
		configNamespace:   configNamespace,
		clientset:         clientset,
		runtimeAsync:      runtimeAsync,
		registryAsync:     registryAsync,
		nodeIPAsync:       nodeIPAsync,
		dynamic:           di,
		drm:               drm,
	}
}

func ServiceURL(service *v1.Service, ip NodeIP) (*url.URL, error) {
	status := service.Status

	lbStatus := status.LoadBalancer

	if len(service.Spec.Ports) == 0 {
		return nil, nil
	}

	portSpec := service.Spec.Ports[0]
	port := portSpec.Port
	nodePort := portSpec.NodePort

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
			return nil, errors.Wrap(err, "ServiceURL: malformed url")
		}
		return url, nil
	}

	// If the node has an IP that we can hit, we can also look
	// at the NodePort. This is mostly useful for Minikube.
	if ip != "" && nodePort != 0 {
		url, err := url.Parse(fmt.Sprintf("http://%s:%d/", ip, nodePort))
		if err != nil {
			return nil, errors.Wrap(err, "ServiceURL: malformed url")
		}
		return url, nil
	}

	return nil, nil
}

func (k K8sClient) Upsert(ctx context.Context, entities []K8sEntity) ([]K8sEntity, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-k8sUpsert")
	defer span.Finish()

	result := make([]K8sEntity, 0, len(entities))

	mutable, immutable := MutableAndImmutableEntities(entities)

	if len(mutable) > 0 {
		newEntities, err := k.applyEntitiesAndMaybeForce(ctx, mutable)
		if err != nil {
			return nil, err
		}
		result = append(result, newEntities...)
	}

	if len(immutable) > 0 {
		newEntities, err := k.forceReplaceEntities(ctx, immutable)
		if err != nil {
			return nil, err
		}
		result = append(result, newEntities...)
	}

	return result, nil
}

func (k K8sClient) forceReplaceEntities(ctx context.Context, entities []K8sEntity) ([]K8sEntity, error) {
	stdout, stderr, err := k.actOnEntities(ctx, []string{"replace", "-o", "yaml", "--force"}, entities)
	if err != nil {
		return nil, errors.Wrapf(err, "kubectl replace:\nstderr: %s", stderr)
	}

	return parseYAMLFromStringWithDeletedResources(stdout)
}

// applyEntitiesAndMaybeForce `kubectl apply`'s the given entities, and if the call fails with
// an immutible field error, attempts to `replace --force` them.
func (k K8sClient) applyEntitiesAndMaybeForce(ctx context.Context, entities []K8sEntity) ([]K8sEntity, error) {
	stdout, stderr, err := k.actOnEntities(ctx, []string{"apply", "-o", "yaml"}, entities)
	if err != nil {
		shouldTryReplace := maybeImmutableFieldStderr(stderr)

		if !shouldTryReplace {
			return nil, errors.Wrapf(err, "kubectl apply:\nstderr: %s", stderr)
		}

		// If the kubectl apply failed due to an immutable field, fall back to kubectl delete && kubectl apply
		// NOTE(maia): this is equivalent to `kubecutl replace --force`, but will ensure that all
		// dependant pods get deleted rather than orphaned. We WANT these pods to be deleted
		// and recreated so they have all the new labels, etc. of their controlling k8s entity.
		logger.Get(ctx).Infof("Falling back to 'kubectl delete && apply' on immutable field error")
		_, stderr, err = k.actOnEntities(ctx, []string{"delete"}, entities)
		if err != nil {
			return nil, errors.Wrapf(err, "kubectl delete (as part of delete && apply):\nstderr: %s", stderr)
		}
		stdout, stderr, err = k.actOnEntities(ctx, []string{"apply", "-o", "yaml"}, entities)
		if err != nil {
			return nil, errors.Wrapf(err, "kubectl apply (as part of delete && apply):\nstderr: %s", stderr)
		}
	}

	return ParseYAMLFromString(stdout)
}

func (k K8sClient) ConnectedToCluster(ctx context.Context) error {
	stdout, stderr, err := k.kubectlRunner.exec(ctx, []string{"cluster-info"})
	if err != nil {
		return errors.Wrapf(err, "Unable to connect to cluster via `kubectl cluster-info`:\nstdout: %s\nstderr: %s", stdout, stderr)
	}

	return nil
}

// We're using kubectl, so we only get stderr, not structured errors.
//
// Take a wild guess if the update is failing due to immutable field errors.
//
// This should bias towards false positives (i.e., we think something is an
// immutable field error when it's not).
func maybeImmutableFieldStderr(stderr string) bool {
	return strings.Contains(stderr, validation.FieldImmutableErrorMsg) || ForbiddenFieldsRe.Match([]byte(stderr))
}

// Deletes all given entities.
//
// Currently ignores any "not found" errors, because that seems like the correct
// behavior for our use cases.
func (k K8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	l := logger.Get(ctx)
	for _, e := range entities {
		l.Infof("Deleting via kubectl: %s/%s\n", e.GVK().Kind, e.Name())
	}

	_, stderr, err := k.actOnEntities(ctx, []string{"delete", "--ignore-not-found"}, entities)
	if err != nil {
		return errors.Wrapf(err, "kubectl delete:\nstderr: %s", stderr)
	}
	return nil
}

func (k K8sClient) actOnEntities(ctx context.Context, cmdArgs []string, entities []K8sEntity) (stdout string, stderr string, err error) {
	args := append([]string{}, cmdArgs...)
	args = append(args, "-f", "-")

	rawYAML, err := SerializeSpecYAML(entities)
	if err != nil {
		return "", "", errors.Wrapf(err, "serializeYaml for kubectl %s", cmdArgs)
	}

	return k.kubectlRunner.execWithStdin(ctx, args, rawYAML)
}

func (k K8sClient) GetByReference(ctx context.Context, ref v1.ObjectReference) (K8sEntity, error) {
	group := getGroup(ref)
	kind := ref.Kind
	namespace := ref.Namespace
	name := ref.Name
	resourceVersion := ref.ResourceVersion
	uid := ref.UID
	rm, err := k.drm.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
	if err != nil {
		return K8sEntity{}, errors.Wrapf(err, "error mapping %s/%s", group, kind)
	}

	result, err := k.dynamic.Resource(rm.Resource).Namespace(namespace).Get(name, metav1.GetOptions{
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		return K8sEntity{}, err
	}
	if uid != "" && result.GetUID() != uid {
		return K8sEntity{}, apierrors.NewNotFound(v1.Resource(kind), name)
	}
	return NewK8sEntity(result), nil
}

// Tests whether a string is a valid version for a k8s resource type.
// from https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/#version-priority
// Versions start with a v followed by a number, an optional beta or alpha designation, and optional additional numeric
// versioning information. Broadly, a version string might look like v2 or v2beta1.
var versionRegex = regexp.MustCompile(`^v\d+(?:(?:alpha|beta)(?:\d+)?)?$`)

func getGroup(involvedObject v1.ObjectReference) string {
	// For some types, APIVersion is incorrectly just the group w/ no version, which leads GroupVersionKind to return
	// a value where Group is empty and Version contains the group, so we need to correct for that.
	// An empty Group is valid, though: it's empty for apps in the core group.
	// So, we detect this situation by checking if the version field is valid.

	// this stems from group/version not necessarily being populated at other points in the API. see more info here:
	// https://github.com/kubernetes/client-go/issues/308
	// https://github.com/kubernetes/kubernetes/issues/3030

	gvk := involvedObject.GroupVersionKind()
	group := gvk.Group
	if !versionRegex.MatchString(gvk.Version) {
		group = involvedObject.APIVersion
	}

	return group
}

func ProvideServerVersion(maybeClientset ClientsetOrError) (*version.Info, error) {
	if maybeClientset.Error != nil {
		return nil, maybeClientset.Error
	}
	return maybeClientset.Clientset.Discovery().ServerVersion()
}

type ClientsetOrError struct {
	Clientset *kubernetes.Clientset
	Error     error
}

func ProvideClientset(cfg RESTConfigOrError) ClientsetOrError {
	if cfg.Error != nil {
		return ClientsetOrError{Error: cfg.Error}
	}
	clientset, err := kubernetes.NewForConfig(cfg.Config)
	return ClientsetOrError{Clientset: clientset, Error: err}
}

func ProvideClientConfig() clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{}
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		rules,
		overrides)
}

// The namespace in the kubeconfig.
// Used as a default namespace in some (but not all) client commands.
// https://godoc.org/k8s.io/client-go/tools/clientcmd/api/v1#Context
func ProvideConfigNamespace(clientLoader clientcmd.ClientConfig) Namespace {
	namespace, explicit, err := clientLoader.Namespace()
	if err != nil {
		// If we can't get a namespace from the config, just fail gracefully to the default.
		// If this error indicates a more serious problem, it will get handled downstream.
		return ""
	}

	// TODO(nick): Right now, tilt doesn't provide a namespace flag. If we ever did,
	// we would need to handle explicit namespaces different than implicit ones.
	_ = explicit

	return Namespace(namespace)
}

type RESTConfigOrError struct {
	Config *rest.Config
	Error  error
}

func ProvideRESTConfig(clientLoader clientcmd.ClientConfig) RESTConfigOrError {
	config, err := clientLoader.ClientConfig()
	return RESTConfigOrError{Config: config, Error: err}
}
