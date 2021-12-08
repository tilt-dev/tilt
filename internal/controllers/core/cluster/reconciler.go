package cluster

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	ctrlClient     ctrlclient.Client
	localDockerEnv docker.LocalEnv
	store          store.RStore

	// TODO(nick): We should have all reconcilers share a client cache, and
	// be able to wait on the connection to complete.
	connections map[types.NamespacedName]*connection
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{})
	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, store store.RStore, localDockerEnv docker.LocalEnv) *Reconciler {
	return &Reconciler{
		ctrlClient:     ctrlClient,
		store:          store,
		localDockerEnv: localDockerEnv,
		connections:    make(map[types.NamespacedName]*connection),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName
	ctx = logger.CtxWithLogHandler(ctx, logWriter{store: r.store})

	var obj v1alpha1.Cluster
	err := r.ctrlClient.Get(ctx, nn, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		delete(r.connections, nn)
		return ctrl.Result{}, err
	}

	connection, hasConnection := r.connections[nn]
	if hasConnection {
		// If the spec changed, delete the connection and recreate it.
		if !apicmp.DeepEqual(connection.spec, obj.Spec) {
			delete(r.connections, nn)
			connection = nil
			hasConnection = false
		}
	}

	if !hasConnection {
		// Create the initial connection to the cluster.
		if obj.Spec.Connection != nil && obj.Spec.Connection.Kubernetes != nil {
			connection = r.createKubernetesConnection(ctx, obj.Spec.Connection.Kubernetes)
		} else if obj.Spec.Connection != nil && obj.Spec.Connection.Docker != nil {
			connection = r.createDockerConnection(ctx, obj.Spec.Connection.Docker)
		}
		connection.createdAt = time.Now()
		connection.spec = obj.Spec
	}

	// TODO(nick): Populate the cluster architecture.
	status := connection.toStatus()
	err = r.maybeUpdateStatus(ctx, &obj, status)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Creates a docker connection from the spec.
func (r *Reconciler) createDockerConnection(ctx context.Context, obj *v1alpha1.DockerClusterConnection) *connection {
	// If no Host is specified, use the default Env from environment variables.
	env := docker.Env(r.localDockerEnv)
	if obj.Host != "" {
		env = docker.Env{Host: obj.Host}
	}

	client := docker.NewDockerClient(ctx, env)
	err := client.CheckConnected()
	if err != nil {
		return &connection{error: err.Error()}
	}
	return &connection{dockerClient: client}
}

// Creates a Kubernetes connection from the spec.
//
// The Kubernetes Client APIs are really defined for automatic dependency injection.
// (as opposed to the Kuberentes convention of nested factory structs.)
//
// If you have to edit the below, it's easier to let wire generate the
// factory code for you, then adapt it here.
func (r *Reconciler) createKubernetesConnection(ctx context.Context, obj *v1alpha1.KubernetesClusterConnection) *connection {
	k8sKubeContextOverride := k8s.KubeContextOverride(obj.Context)
	k8sNamespaceOverride := k8s.NamespaceOverride(obj.Namespace)
	clientConfig := k8s.ProvideClientConfig(k8sKubeContextOverride, k8sNamespaceOverride)
	apiConfig, err := k8s.ProvideKubeConfig(clientConfig, k8sKubeContextOverride)
	if err != nil {
		return &connection{error: err.Error()}
	}
	env := k8s.ProvideEnv(ctx, apiConfig)
	restConfigOrError := k8s.ProvideRESTConfig(clientConfig)

	clientsetOrError := k8s.ProvideClientset(restConfigOrError)
	portForwardClient := k8s.ProvidePortForwardClient(restConfigOrError, clientsetOrError)
	namespace := k8s.ProvideConfigNamespace(clientConfig)
	kubeContext, err := k8s.ProvideKubeContext(apiConfig)
	if err != nil {
		return &connection{error: err.Error()}
	}
	minikubeClient := k8s.ProvideMinikubeClient(kubeContext)
	client := k8s.ProvideK8sClient(env, restConfigOrError, clientsetOrError, portForwardClient, namespace, minikubeClient, clientConfig)
	_, err = client.CheckConnected(ctx)
	if err != nil {
		return &connection{error: err.Error()}
	}
	return &connection{k8sClient: client}
}

func (r *Reconciler) maybeUpdateStatus(ctx context.Context, obj *v1alpha1.Cluster, newStatus v1alpha1.ClusterStatus) error {
	if apicmp.DeepEqual(obj.Status, newStatus) {
		return nil
	}

	updated := obj.DeepCopy()
	updated.Status = newStatus
	err := r.ctrlClient.Status().Update(ctx, updated)
	if err != nil {
		return fmt.Errorf("updating cluster %s status: %v", obj.Name, err)
	}

	if newStatus.Error != "" && obj.Status.Error != newStatus.Error {
		logger.Get(ctx).Errorf("Cluster status error: %v", newStatus.Error)
	}
	return nil
}

type connection struct {
	spec         v1alpha1.ClusterSpec
	dockerClient docker.Client
	k8sClient    k8s.Client
	error        string
	createdAt    time.Time
}

func (c *connection) toStatus() v1alpha1.ClusterStatus {
	return v1alpha1.ClusterStatus{
		Error: c.error,
	}
}

type logWriter struct {
	store store.RStore
}

func (w logWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(model.MainTiltfileManifestName, "cluster", level, fields, p))
	return nil
}
