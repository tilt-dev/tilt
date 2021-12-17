package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/kube"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubectl/pkg/cmd/wait"

	// Client auth plugins! They will auto-init if we import them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/logger"
)

// Due to the way the Kubernetes apiserver works, there's no easy way to
// distinguish between "server is taking a long time to respond because it's
// gone" and "server is taking a long time to respond because it has a slow auth
// plugin".
//
// So our health check timeout is a bit longer than we'd like.
const healthCheckTimeout = 5 * time.Second

type Namespace string
type NamespaceOverride string
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
	InformerSet

	// Updates the entities, creating them if necessary.
	//
	// Tries to update them in-place if possible. But for certain resource types,
	// we might need to fallback to deleting and re-creating them.
	//
	// Returns entities in the order that they were applied (which may be different
	// than they were passed in) and with UUIDs from the Kube API
	Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error)

	// Delete all given entities, optionally waiting for them to be fully deleted.
	//
	// Currently ignores any "not found" errors, because that seems like the correct
	// behavior for our use cases.
	Delete(ctx context.Context, entities []K8sEntity, wait bool) error

	GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (metav1.Object, error)
	ListMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]metav1.Object, error)

	// Streams the container logs
	ContainerLogs(ctx context.Context, podID PodID, cName container.Name, n Namespace, startTime time.Time) (io.ReadCloser, error)

	// Opens a tunnel to the specified pod+port. Returns the tunnel's local port and a function that closes the tunnel
	CreatePortForwarder(ctx context.Context, namespace Namespace, podID PodID, optionalLocalPort, remotePort int, host string) (PortForwarder, error)

	WatchMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) (<-chan metav1.Object, error)

	ContainerRuntime(ctx context.Context) container.Runtime

	// Some clusters support a local image registry that we can push to.
	LocalRegistry(ctx context.Context) container.Registry

	// Some clusters support a node IP where all servers are reachable.
	NodeIP(ctx context.Context) NodeIP

	Exec(ctx context.Context, podID PodID, cName container.Name, n Namespace, cmd []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) error

	// Returns version information about the apiserver, or an error if we're not connected.
	CheckConnected(ctx context.Context) (*version.Info, error)
}

type RESTMapper interface {
	meta.RESTMapper
	Reset()
}

type K8sClient struct {
	InformerSet

	env               Env
	core              apiv1.CoreV1Interface
	restConfig        *rest.Config
	portForwardClient PortForwardClient
	configNamespace   Namespace
	clientset         kubernetes.Interface
	discovery         discovery.CachedDiscoveryInterface
	dynamic           dynamic.Interface
	metadata          metadata.Interface
	runtimeAsync      *runtimeAsync
	registryAsync     *registryAsync
	nodeIPAsync       *nodeIPAsync
	drm               RESTMapper
	clientLoader      clientcmd.ClientConfig
	resourceClient    ResourceClient
}

var _ Client = &K8sClient{}

func ProvideK8sClient(
	env Env,
	maybeRESTConfig RESTConfigOrError,
	maybeClientset ClientsetOrError,
	pfClient PortForwardClient,
	configNamespace Namespace,
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

	meta, err := metadata.NewForConfig(restConfig)
	if err != nil {
		return &explodingClient{err: err}
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return &explodingClient{fmt.Errorf("unable to create discovery client: %v", err)}
	}

	discovery := memory.NewMemCacheClient(discoveryClient)

	drm := restmapper.NewDeferredDiscoveryRESTMapper(discovery)

	c := &K8sClient{
		InformerSet: newInformerSet(clientset, di),

		env:               env,
		core:              core,
		restConfig:        restConfig,
		portForwardClient: pfClient,
		discovery:         discovery,
		configNamespace:   configNamespace,
		clientset:         clientset,
		runtimeAsync:      runtimeAsync,
		registryAsync:     registryAsync,
		nodeIPAsync:       nodeIPAsync,
		dynamic:           di,
		drm:               drm,
		metadata:          meta,
		clientLoader:      clientLoader,
	}
	c.resourceClient = newResourceClient(c)
	return c
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

func (k *K8sClient) ToRESTConfig() (*rest.Config, error) {
	return rest.CopyConfig(k.restConfig), nil
}

func (k *K8sClient) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return k.discovery, nil
}

// Loosely adapted from ctlptl.
func (k *K8sClient) CheckConnected(ctx context.Context) (*version.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	discoClient, err := k.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	restClient := discoClient.RESTClient()
	if restClient == nil {
		return discoClient.ServerVersion()
	}

	body, err := restClient.Get().AbsPath("/version").Do(ctx).Raw()
	if err != nil {
		return nil, err
	}
	var info version.Info
	err = json.Unmarshal(body, &info)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the server version: %v", err)
	}
	return &info, nil
}

func (k *K8sClient) ToRESTMapper() (meta.RESTMapper, error) {
	return k.drm, nil
}
func (k *K8sClient) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return k.clientLoader
}

func (k *K8sClient) Upsert(ctx context.Context, entities []K8sEntity, timeout time.Duration) ([]K8sEntity, error) {
	result := make([]K8sEntity, 0, len(entities))

	mutable, immutable := MutableAndImmutableEntities(entities)

	for _, e := range mutable {
		innerCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		newEntity, err := k.escalatingUpdate(innerCtx, e)
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

		newEntities, err := k.deleteAndCreateEntity(innerCtx, e)
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

// Update an entity like kubectl apply does.
//
// This is the "best" way to apply a change.
// It will do a 3-way merge to update the spec in the least intrusive way.
func (k *K8sClient) applyEntity(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	resources, err := k.prepareUpdateList(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "kubernetes apply")
	}

	result, err := k.resourceClient.Apply(resources)
	if err != nil {
		return nil, err
	}

	return k.helmResultToEntities(result)
}

// Update an entity like kubectl create/replace does.
//
// This uses a PUT HTTP call to replace one entity with another.
//
// It's not as good as apply, because it will wipe out bookkeeping
// that other controllers have added.
//
// But in cases where the entity is too big to do a 3-way merge,
// this is the next best option.
func (k *K8sClient) createOrReplaceEntity(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	resources, err := k.prepareUpdateList(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "kubernetes upsert")
	}

	result, err := k.resourceClient.CreateOrReplace(resources)
	if err != nil {
		return nil, err
	}

	return k.helmResultToEntities(result)
}

// Delete and create an entity.
//
// This is the most intrusive way to perform an update,
// because any children of the object will be deleted by the controller.
//
// Some objects in the Kubernetes ecosystem are immutable, so need
// this approach as a last resort.
func (k *K8sClient) deleteAndCreateEntity(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	resources, err := k.prepareUpdateList(ctx, entity)
	if err != nil {
		return nil, errors.Wrap(err, "kubernetes delete and re-create")
	}

	result, err := k.deleteAndCreate(resources)
	if err != nil {
		return nil, err
	}

	return k.helmResultToEntities(result)
}

// Make sure the type exists and create a ResourceList to help update it.
func (k *K8sClient) prepareUpdateList(ctx context.Context, e K8sEntity) (kube.ResourceList, error) {
	_, err := k.forceDiscovery(ctx, e.GVK())
	if err != nil {
		return nil, err
	}

	return k.buildResourceList(ctx, e)
}

// Build a ResourceList usable by our helm client for interacting with a resource.
//
// Although the underlying API encourages you to batch these together (for
// better parallelization), we've found that it's more robust to handle entities
// individually to ensure an error in one doesn't affect the others (and the
// real bottleneck isn't in building).
func (k *K8sClient) buildResourceList(ctx context.Context, e K8sEntity) (kube.ResourceList, error) {
	rawYAML, err := SerializeSpecYAMLToBuffer([]K8sEntity{e})
	if err != nil {
		return nil, err
	}

	resources, err := k.resourceClient.Build(rawYAML, false)
	if err != nil {
		return nil, err
	}

	return resources, nil
}

func (k *K8sClient) helmResultToEntities(result *kube.Result) ([]K8sEntity, error) {
	entities := []K8sEntity{}
	for _, info := range result.Created {
		entities = append(entities, NewK8sEntity(info.Object))
	}
	for _, info := range result.Updated {
		entities = append(entities, NewK8sEntity(info.Object))
	}

	// Helm parses the results as unstructured info, but Tilt needs them parsed with the current
	// API scheme. The easiest way to do this is to serialize them to yaml and re-parse again.
	buf, err := SerializeSpecYAMLToBuffer(entities)
	if err != nil {
		return nil, errors.Wrap(err, "reading kubernetes result")
	}

	parsed, err := ParseYAML(buf)
	if err != nil {
		return nil, errors.Wrap(err, "parsing kubernetes result")
	}
	return parsed, nil
}

func (k *K8sClient) deleteAndCreate(list kube.ResourceList) (*kube.Result, error) {
	// Delete is destructive, so clone first.
	toDelete := kube.ResourceList{}
	for _, r := range list {
		rClone := *r
		rClone.Object = r.Object.DeepCopyObject()
		toDelete = append(toDelete, &rClone)
	}

	_, errs := k.resourceClient.Delete(toDelete)
	for _, err := range errs {
		if isNotFoundError(err) {
			continue
		}
		return nil, errors.Wrap(err, "kubernetes delete")
	}

	// ensure the delete has finished before attempting to recreate
	k.waitForDelete(list)

	result, err := k.resourceClient.Create(list)
	if err != nil {
		return nil, errors.Wrap(err, "kubernetes create")
	}
	return result, nil
}

// Update a resource in-place, starting with the least intrusive
// update strategy and escalating into the most intrusive strategy.
func (k *K8sClient) escalatingUpdate(ctx context.Context, entity K8sEntity) ([]K8sEntity, error) {
	fallback := false
	result, err := k.applyEntity(ctx, entity)
	if err != nil {
		msg, match := maybeTooLargeError(err)
		if match {
			fallback = true
			logger.Get(ctx).Infof("Updating %q failed: %s", entity.Name(), msg)
			logger.Get(ctx).Infof("Attempting to create or replace")
			result, err = k.createOrReplaceEntity(ctx, entity)
		}
	}

	if err != nil {
		isImmutable := maybeImmutableFieldStderr(err.Error())
		if isImmutable {
			fallback = true
			logger.Get(ctx).Infof("Updating %q failed: immutable field error", entity.Name())
			logger.Get(ctx).Infof("Attempting to delete and re-create")
			result, err = k.deleteAndCreateEntity(ctx, entity)
		}
	}

	if err != nil {
		return nil, err
	}
	if fallback {
		logger.Get(ctx).Infof("Updating %q succeeded!", entity.Name())
	}
	return result, nil
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
//
// We've also seen this reported differently, with a 413 HTTP error.
// https://github.com/tilt-dev/tilt/issues/5279
func maybeTooLargeError(err error) (string, bool) {
	statusErr, isStatusErr := err.(*apiErrors.StatusError)
	if isStatusErr && statusErr.ErrStatus.Code == http.StatusRequestEntityTooLarge {
		return err.Error(), true
	}

	stderr := err.Error()
	for _, line := range strings.Split(stderr, "\n") {
		if MetadataAnnotationsTooLongRe.MatchString(line) {
			return line, true
		}
		if strings.Contains(line, "the server responded with the status code 413") {
			return line, true
		}
	}

	return "", false
}

// Deletes all given entities.
//
// Currently ignores any "not found" errors, because that seems like the correct
// behavior for our use cases.
func (k *K8sClient) Delete(ctx context.Context, entities []K8sEntity, wait bool) error {
	l := logger.Get(ctx)
	l.Infof("Deleting kubernetes objects:")
	for _, e := range entities {
		l.Infof("→ %s/%s", e.GVK().Kind, e.Name())
	}

	var resources kube.ResourceList
	for _, e := range entities {
		resourceList, err := k.buildResourceList(ctx, e)
		if utilerrors.FilterOut(err, isMissingKindError) != nil {
			return errors.Wrap(err, "kubernetes delete")
		}
		resources = append(resources, resourceList...)
	}

	_, errs := k.resourceClient.Delete(resources)
	for _, err := range errs {
		if err == nil || isNotFoundError(err) {
			continue
		}

		return errors.Wrap(err, "kubernetes delete")
	}

	if wait {
		k.waitForDelete(resources)
	}

	return nil
}

func (k *K8sClient) forceDiscovery(ctx context.Context, gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	rm, err := k.drm.RESTMapping(gvk.GroupKind(), gvk.Version)
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

		rm, err = k.drm.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return nil, errors.Wrapf(err, "error mapping %s/%s", gvk.Group, gvk.Kind)
		}
	}
	return rm, nil
}

func (k *K8sClient) waitForDelete(list kube.ResourceList) {
	var wg sync.WaitGroup
	for _, r := range list {
		wg.Add(1)
		go func(resourceInfo *resource.Info) {
			waitOpt := &wait.WaitOptions{
				DynamicClient: k.dynamic,
				IOStreams:     genericclioptions.NewTestIOStreamsDiscard(),
				Timeout:       30 * time.Second,
				ForCondition:  "delete",
			}

			_, _, _ = wait.IsDeleted(resourceInfo, waitOpt)
			wg.Done()
		}(r)
	}
	wg.Wait()
}

func (k *K8sClient) ListMeta(ctx context.Context, gvk schema.GroupVersionKind, ns Namespace) ([]metav1.Object, error) {
	mapping, err := k.forceDiscovery(ctx, gvk)
	if err != nil {
		return nil, err
	}

	gvr := mapping.Resource
	isRoot := mapping.Scope != nil && mapping.Scope.Name() == meta.RESTScopeNameRoot
	var metaList *metav1.PartialObjectMetadataList
	if isRoot {
		metaList, err = k.metadata.Resource(gvr).List(ctx, metav1.ListOptions{})
	} else {
		metaList, err = k.metadata.Resource(gvr).Namespace(ns.String()).List(ctx, metav1.ListOptions{})
	}

	if err != nil {
		return nil, err
	}

	// type conversion
	result := make([]metav1.Object, len(metaList.Items))
	for i, meta := range metaList.Items {
		m := meta.ObjectMeta
		result[i] = &m
	}
	return result, nil
}

func (k *K8sClient) GetMetaByReference(ctx context.Context, ref v1.ObjectReference) (metav1.Object, error) {
	gvk := ReferenceGVK(ref)
	mapping, err := k.forceDiscovery(ctx, gvk)
	if err != nil {
		return nil, err
	}

	gvr := mapping.Resource
	namespace := ref.Namespace
	name := ref.Name
	resourceVersion := ref.ResourceVersion
	uid := ref.UID

	typeAndMeta, err := k.metadata.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{
		ResourceVersion: resourceVersion,
	})
	if err != nil {
		return nil, err
	}
	meta := typeAndMeta.ObjectMeta
	if uid != "" && meta.UID != uid {
		return nil, apierrors.NewNotFound(v1.Resource(gvr.Resource), name)
	}
	return &meta, nil
}

// Tests whether a string is a valid version for a k8s resource type.
// from https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definition-versioning/#version-priority
// Versions start with a v followed by a number, an optional beta or alpha designation, and optional additional numeric
// versioning information. Broadly, a version string might look like v2 or v2beta1.
var versionRegex = regexp.MustCompile(`^v\d+(?:(?:alpha|beta)(?:\d+)?)?$`)

func ReferenceGVK(involvedObject v1.ObjectReference) schema.GroupVersionKind {
	// For some types, APIVersion is incorrectly just the group w/ no version, which leads GroupVersionKind to return
	// a value where Group is empty and Version contains the group, so we need to correct for that.
	// An empty Group is valid, though: it's empty for apps in the core group.
	// So, we detect this situation by checking if the version field is valid.

	// this stems from group/version not necessarily being populated at other points in the API. see more info here:
	// https://github.com/kubernetes/client-go/issues/308
	// https://github.com/kubernetes/kubernetes/issues/3030

	gvk := involvedObject.GroupVersionKind()
	if !versionRegex.MatchString(gvk.Version) {
		gvk.Group = involvedObject.APIVersion
		gvk.Version = ""
	}

	return gvk
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

func ProvideClientConfig(contextOverride KubeContextOverride, nsFlag NamespaceOverride) clientcmd.ClientConfig {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	rules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	overrides := &clientcmd.ConfigOverrides{
		CurrentContext: string(contextOverride),
		Context: clientcmdapi.Context{
			Namespace: string(nsFlag),
		},
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
