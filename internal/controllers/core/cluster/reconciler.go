package cluster

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/clusters"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const ArchUnknown string = "unknown"

type Reconciler struct {
	globalCtx   context.Context
	ctrlClient  ctrlclient.Client
	store       store.RStore
	connManager *ConnectionManager

	localDockerEnv      docker.LocalEnv
	dockerClientFactory DockerClientFactory

	k8sClientFactory KubernetesClientFactory
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{})
	return b, nil
}

func NewReconciler(
	globalCtx context.Context,
	ctrlClient ctrlclient.Client,
	store store.RStore,
	connManager *ConnectionManager,
	localDockerEnv docker.LocalEnv,
	dockerClientFactory DockerClientFactory,
	k8sClientFactory KubernetesClientFactory,
) *Reconciler {
	return &Reconciler{
		globalCtx:           globalCtx,
		ctrlClient:          ctrlClient,
		store:               store,
		connManager:         connManager,
		localDockerEnv:      localDockerEnv,
		dockerClientFactory: dockerClientFactory,
		k8sClientFactory:    k8sClientFactory,
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName
	ctx = store.WithManifestLogHandler(ctx, r.store, model.MainTiltfileManifestName, "cluster")

	var obj v1alpha1.Cluster
	err := r.ctrlClient.Get(ctx, nn, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		r.store.Dispatch(clusters.NewClusterDeleteAction(request.Name))
		r.connManager.delete(nn)
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.store.Dispatch(clusters.NewClusterUpsertAction(&obj))

	conn, hasConnection := r.connManager.load(nn)
	if hasConnection {
		// If the spec changed, delete the connection and recreate it.
		if !apicmp.DeepEqual(conn.spec, obj.Spec) {
			r.connManager.delete(nn)
			conn = connection{}
			hasConnection = false
		}
	}

	if !hasConnection {
		// Create the initial connection to the cluster.
		if obj.Spec.Connection != nil && obj.Spec.Connection.Kubernetes != nil {
			conn = r.createKubernetesConnection(obj.Spec.Connection.Kubernetes)
		} else if obj.Spec.Connection != nil && obj.Spec.Connection.Docker != nil {
			conn = r.createDockerConnection(obj.Spec.Connection.Docker)
		}
		conn.createdAt = time.Now()
		conn.spec = obj.Spec
	}

	if conn.error == "" && conn.arch == "" {
		if conn.k8sClient != nil {
			conn.arch = r.readKubernetesArch(ctx, conn.k8sClient)
		} else if conn.dockerClient != nil {
			conn.arch = r.readDockerArch(ctx, conn.dockerClient)
		}
	}

	r.connManager.store(nn, conn)

	status := conn.toStatus()
	err = r.maybeUpdateStatus(ctx, &obj, status)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Creates a docker connection from the spec.
func (r *Reconciler) createDockerConnection(obj *v1alpha1.DockerClusterConnection) connection {
	// If no Host is specified, use the default Env from environment variables.
	env := docker.Env(r.localDockerEnv)
	if obj.Host != "" {
		env = docker.Env{Host: obj.Host}
	}

	client, err := r.dockerClientFactory.New(r.globalCtx, env)
	if err != nil {
		return connection{connType: connectionTypeDocker, error: err.Error()}
	}

	return connection{connType: connectionTypeDocker, dockerClient: client}
}

// Creates a Kubernetes connection from the spec.
func (r *Reconciler) createKubernetesConnection(obj *v1alpha1.KubernetesClusterConnection) connection {
	k8sKubeContextOverride := k8s.KubeContextOverride(obj.Context)
	k8sNamespaceOverride := k8s.NamespaceOverride(obj.Namespace)
	client, err := r.k8sClientFactory.New(r.globalCtx, k8sKubeContextOverride, k8sNamespaceOverride)
	if err != nil {
		return connection{connType: connectionTypeK8s, error: err.Error()}
	}
	return connection{connType: connectionTypeK8s, k8sClient: client}
}

// Reads the arch from a kubernetes cluster, or "unknown" if we can't
// figure out the architecture.
//
// Note that it's normal that users may not have access to the kubernetes
// arch if there are RBAC rules restricting read access on nodes.
//
// We only need to read SOME arch that the cluster supports.
func (r *Reconciler) readKubernetesArch(ctx context.Context, client k8s.Client) string {
	nodeMetas, err := client.ListMeta(ctx, schema.GroupVersionKind{Version: "v1", Kind: "Node"}, "")
	if err != nil || len(nodeMetas) == 0 {
		return ArchUnknown
	}

	// https://github.com/kubernetes/enhancements/blob/0e4d5df19d396511fe41ed0860b0ab9b96f46a2d/keps/sig-node/793-node-os-arch-labels/README.md
	// https://kubernetes.io/docs/reference/labels-annotations-taints/#kubernetes-io-arch
	arch := nodeMetas[0].GetLabels()["kubernetes.io/arch"]
	if arch == "" {
		arch = nodeMetas[0].GetLabels()["beta.kubernetes.io/arch"]
	}

	if arch == "" {
		return ArchUnknown
	}
	return arch
}

// Reads the arch from a Docker cluster, or "unknown" if we can't
// figure out the architecture.
func (r *Reconciler) readDockerArch(ctx context.Context, client docker.Client) string {
	arch := client.ServerVersion().Arch
	if arch == "" {
		return ArchUnknown
	}
	return arch
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

	r.reportConnectionEvent(ctx, updated)

	return nil
}

func (r *Reconciler) reportConnectionEvent(ctx context.Context, cluster *v1alpha1.Cluster) {
	tags := make(map[string]string)

	if cluster.Spec.Connection != nil {
		if cluster.Spec.Connection.Kubernetes != nil {
			tags["type"] = "kubernetes"
		} else if cluster.Spec.Connection.Docker != nil {
			tags["type"] = "docker"
		}
	}

	if cluster.Status.Arch != "" {
		tags["arch"] = cluster.Status.Arch
	}

	if cluster.Status.Error == "" {
		tags["status"] = "connected"
	} else {
		tags["status"] = "error"
	}

	analytics.Get(ctx).Incr("api.cluster.connect", tags)
}

func (c *connection) toStatus() v1alpha1.ClusterStatus {
	var connectedAt *metav1.MicroTime
	if c.error == "" && !c.createdAt.IsZero() {
		t := apis.NewMicroTime(c.createdAt)
		connectedAt = &t
	}

	return v1alpha1.ClusterStatus{
		Error:       c.error,
		Arch:        c.arch,
		ConnectedAt: connectedAt,
	}
}
