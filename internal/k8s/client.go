package k8s

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/browser"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Namespace string
type PodID string
type NodeID string
type ServiceName string
type KubeContext string
type KubeContextOverride string

// NOTE(nick): This isn't right. DefaultNamespace is a function of your kubectl context.
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
	Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error)

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
	drm               *restmapper.DeferredDiscoveryRESTMapper
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
	mkClient MinikubeClient,
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
	nodeIPAsync := newNodeIPAsync(env, mkClient)

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

func timeoutError(timeout time.Duration) error {
	return errors.New(fmt.Sprintf("Killed kubectl. Hit timeout of %v.", timeout))
}

func (k K8sClient) Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error) {
	result := make([]K8sEntity, 0, len(entities))

	mutable, immutable := MutableAndImmutableEntities(entities)

	for _, e := range mutable {
		innerCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		newEntity, err := k.applyEntityAndMaybeForce(innerCtx, e)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, timeoutError(timeout)
			}
			return nil, err
		}
		result = append(result, newEntity...)
	}

	for _, e := range immutable {
		innerCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		newEntities, err := k.forceReplaceEntity(innerCtx, e)
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return nil, timeoutError(timeout)
			}
			return nil, err
		}
		result = append(result, newEntities...)
	}

	return result, nil
}

func (k K8sClient) forceReplaceEntity(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	stdout, stderr, err := k.actOnEntity(ctx, []string{"replace", "-o", "yaml", "--force"}, entity)
	if err != nil {
		return nil, errors.Wrapf(err, "kubectl replace:\nstderr: %s", stderr)
	}

	return parseYAMLFromStringWithDeletedResources(stdout)
}

// applyEntityAndMaybeForce `kubectl apply`'s the given entity, and if the call fails with
// an immutible field error, attempts to `replace --force` it.
func (k K8sClient) applyEntityAndMaybeForce(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	stdout, stderr, err := k.actOnEntity(ctx, []string{"apply", "-o", "yaml"}, entity)
	if err != nil {
		reason, shouldTryReplace := maybeShouldTryReplaceReason(stderr)

		if !shouldTryReplace {
			return nil, errors.Wrapf(err, "kubectl apply:\nstderr: %s", stderr)
		}

		// NOTE(maia): we don't use `kubecutl replace --force`, because we want to ensure that all
		// dependant pods get deleted rather than orphaned. We WANT these pods to be deleted
		// and recreated so they have all the new labels, etc. of their controlling k8s entity.
		logger.Get(ctx).Infof("Applying %s failed. Retrying with 'kubectl delete && create': %s", entity.Name(), reason)
		// --ignore-not-found because, e.g., if we fell back due to large metadata.annotations, the object might not exist
		_, stderr, err = k.actOnEntity(ctx, []string{"delete", "--ignore-not-found=true"}, entity)
		if err != nil {
			return nil, errors.Wrapf(err, "kubectl delete (as part of delete && create):\nstderr: %s", stderr)
		}
		stdout, stderr, err = k.actOnEntity(ctx, []string{"create", "-o", "yaml"}, entity)
		if err != nil {
			return nil, errors.Wrapf(err, "kubectl create (as part of delete && create):\nstderr: %s", stderr)
		}
		logger.Get(ctx).Infof("Succeeded!")
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

var MetadataAnnotationsTooLongRe = regexp.MustCompile(`metadata.annotations: Too long: must have at most \d+ bytes.*`)

// kubectl apply sets an annotation containing the object's previous configuration.
// However, annotations have a max size of 256k. Large objects such as configmaps can exceed 256k, which makes
// apply unusable, so we need to fall back to delete/create
// https://github.com/kubernetes/kubectl/issues/712
func maybeAnnotationsTooLong(stderr string) (string, bool) {
	for _, line := range strings.Split(stderr, "\n") {
		if MetadataAnnotationsTooLongRe.MatchString(line) {
			return line, true
		}
	}

	return "", false
}

func maybeShouldTryReplaceReason(stderr string) (string, bool) {
	if maybeImmutableFieldStderr(stderr) {
		return "immutable field error", true
	} else if msg, match := maybeAnnotationsTooLong(stderr); match {
		return fmt.Sprintf("%s (https://github.com/kubernetes/kubectl/issues/712)", msg), true
	}

	return "", false
}

// Deletes all given entities.
//
// Currently ignores any "not found" errors, because that seems like the correct
// behavior for our use cases.
func (k K8sClient) Delete(ctx context.Context, entities []K8sEntity) error {
	l := logger.Get(ctx)
	l.Infof("Deleting via kubectl:")
	for _, e := range entities {
		l.Infof("→ %s/%s", e.GVK().Kind, e.Name())

		_, stderr, err := k.actOnEntity(ctx, []string{"delete", "--ignore-not-found"}, e)
		if err != nil {
			return errors.Wrapf(err, "kubectl delete:\nstderr: %s", stderr)
		}
	}
	return nil
}

func (k K8sClient) actOnEntity(ctx context.Context, cmdArgs []string, entity K8sEntity) (stdout string, stderr string, err error) {
	args := append([]string{}, cmdArgs...)
	args = append(args, "-f", "-")

	rawYAML, err := SerializeSpecYAML([]K8sEntity{entity})
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
		// The REST mapper doesn't have any sort of internal invalidation
		// mechanism. So if the user applies a CRD (i.e., changing the available
		// api resources), the REST mapper won't discover the new types.
		//
		// https://github.com/kubernetes/kubernetes/issues/75383
		//
		// But! When Tilt requests a resource by reference, we know in advance that
		// it must exist, and therefore, its type must exist.  So we can safely
		// reset the REST mapper and retry, so that it discovers the types.
		k.drm.Reset()

		rm, err = k.drm.RESTMapping(schema.GroupKind{Group: group, Kind: kind})
		if err != nil {
			return K8sEntity{}, errors.Wrapf(err, "error mapping %s/%s", group, kind)
		}
	}

	result, err := k.dynamic.Resource(rm.Resource).Namespace(namespace).Get(ctx, name, metav1.GetOptions{
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

func ProvideClientConfig(contextOverride KubeContextOverride) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: string(contextOverride),
	}
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
