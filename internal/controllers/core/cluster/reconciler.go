package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/jonboulle/clockwork"
	"github.com/spf13/afero"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/clientcmd/api/latest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/hud/server"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/clusters"
	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

const ArchUnknown string = "unknown"

const (
	clientInitBackoff        = 30 * time.Second
	clientHealthPollInterval = 15 * time.Second
)

type Reconciler struct {
	globalCtx           context.Context
	ctrlClient          ctrlclient.Client
	store               store.RStore
	requeuer            *indexer.Requeuer
	clock               clockwork.Clock
	connManager         *ConnectionManager
	base                xdg.Base
	apiServerName       model.APIServerName
	localDockerEnv      docker.LocalEnv
	dockerClientFactory DockerClientFactory
	k8sClientFactory    KubernetesClientFactory
	wsList              *server.WebsocketList
	clusterHealth       *clusterHealthMonitor
	filesystem          afero.Fs
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{}).
		WatchesRawSource(r.requeuer)
	return b, nil
}

func NewReconciler(
	globalCtx context.Context,
	ctrlClient ctrlclient.Client,
	store store.RStore,
	clock clockwork.Clock,
	connManager *ConnectionManager,
	localDockerEnv docker.LocalEnv,
	dockerClientFactory DockerClientFactory,
	k8sClientFactory KubernetesClientFactory,
	wsList *server.WebsocketList,
	base xdg.Base,
	apiServerName model.APIServerName,
	filesystem afero.Fs,
) *Reconciler {
	requeuer := indexer.NewRequeuer()

	return &Reconciler{
		globalCtx:           globalCtx,
		ctrlClient:          ctrlClient,
		store:               store,
		clock:               clock,
		requeuer:            requeuer,
		connManager:         connManager,
		localDockerEnv:      localDockerEnv,
		dockerClientFactory: dockerClientFactory,
		k8sClientFactory:    k8sClientFactory,
		wsList:              wsList,
		clusterHealth:       newClusterHealthMonitor(globalCtx, clock, requeuer),
		base:                base,
		apiServerName:       apiServerName,
		filesystem:          filesystem,
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
		r.cleanup(nn)
		r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
			ws.SendClusterUpdate(ctx, nn, nil)
		})
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.store.Dispatch(clusters.NewClusterUpsertAction(&obj))

	clusterRefreshEnabled := obj.Annotations["features.tilt.dev/cluster-refresh"] == "true"
	conn, hasConnection := r.connManager.load(nn)
	// If this is not the first time we've tried to connect to the cluster,
	// only attempt to refresh the connection if the feature is enabled. Not
	// all parts of Tilt use a dynamically-obtained client currently, which
	// can result in erratic behavior if the cluster is not in a usable state
	// at startup but then becomes usable, for example, as some parts of the
	// system will still have k8s.explodingClient.
	if hasConnection && clusterRefreshEnabled {
		// If the spec changed, delete the connection and recreate it.
		if !apicmp.DeepEqual(conn.spec, obj.Spec) {
			r.cleanup(nn)
			conn = connection{}
			hasConnection = false
		} else if conn.initError != "" && r.clock.Now().After(conn.createdAt.Add(clientInitBackoff)) {
			hasConnection = false
		}
	}

	var requeueAfter time.Duration
	if !hasConnection {
		// Create the initial connection to the cluster.
		conn = connection{spec: *obj.Spec.DeepCopy(), createdAt: r.clock.Now()}
		if obj.Spec.Connection != nil && obj.Spec.Connection.Kubernetes != nil {
			conn.connType = connectionTypeK8s
			client, err := r.createKubernetesClient(obj.DeepCopy())
			if err != nil {
				var initError string
				if !clusterRefreshEnabled {
					initError = fmt.Sprintf(
						"Tilt encountered an error connecting to your Kubernetes cluster:"+
							"\n\t%v"+
							"\nYou will need to restart Tilt after resolving the issue.",
						err)
				} else {
					initError = err.Error()
				}
				conn.initError = initError
			} else {
				conn.k8sClient = client
			}
		} else if obj.Spec.Connection != nil && obj.Spec.Connection.Docker != nil {
			conn.connType = connectionTypeDocker
			client, err := r.createDockerClient(obj.Spec.Connection.Docker)
			if err != nil {
				conn.initError = err.Error()
			} else {
				conn.dockerClient = client
			}
		}

		if conn.initError != "" {
			// requeue the cluster Obj so that we can attempt to re-initialize
			requeueAfter = clientInitBackoff
		} else {
			// start monitoring the connection and requeue the Cluster obj
			// for reconciliation if its runtime status changes
			r.clusterHealth.Start(nn, conn)
		}
	}

	r.populateClusterMetadata(ctx, nn, &conn)

	r.connManager.store(nn, conn)

	status := conn.toStatus(r.clusterHealth.GetStatus(nn))
	err = r.maybeUpdateStatus(ctx, &obj, status)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.wsList.ForEach(func(ws *server.WebsocketSubscriber) {
		ws.SendClusterUpdate(ctx, nn, &obj)
	})

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// Creates a docker connection from the spec.
func (r *Reconciler) createDockerClient(obj *v1alpha1.DockerClusterConnection) (docker.Client, error) {
	// If no Host is specified, use the default Env from environment variables.
	env := docker.Env(r.localDockerEnv)
	if obj.Host != "" {
		d, err := client.NewClientWithOpts(client.WithHost(obj.Host))
		env.Client = d
		if err != nil {
			env.Error = err
		}
	}

	client, err := r.dockerClientFactory.New(r.globalCtx, env)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// Creates a Kubernetes client from the spec.
func (r *Reconciler) createKubernetesClient(cluster *v1alpha1.Cluster) (k8s.Client, error) {
	k8sKubeContextOverride := k8s.KubeContextOverride(cluster.Spec.Connection.Kubernetes.Context)
	k8sNamespaceOverride := k8s.NamespaceOverride(cluster.Spec.Connection.Kubernetes.Namespace)
	client, err := r.k8sClientFactory.New(r.globalCtx, k8sKubeContextOverride, k8sNamespaceOverride)
	if err != nil {
		return nil, err
	}
	return client, nil
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
	serverVersion, err := client.ServerVersion(ctx)
	if err != nil {
		return ArchUnknown
	}
	arch := serverVersion.Arch
	if arch == "" {
		return ArchUnknown
	}
	return arch
}

func (r *Reconciler) maybeUpdateStatus(ctx context.Context, obj *v1alpha1.Cluster, newStatus v1alpha1.ClusterStatus) error {
	if apicmp.DeepEqual(obj.Status, newStatus) {
		return nil
	}

	update := obj.DeepCopy()
	oldStatus := update.Status
	update.Status = newStatus
	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return fmt.Errorf("updating cluster %s status: %v", obj.Name, err)
	}

	if newStatus.Error != "" && oldStatus.Error != newStatus.Error {
		logger.Get(ctx).Errorf("Cluster status error: %v", newStatus.Error)
	}

	r.reportConnectionEvent(ctx, update)

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

func (r *Reconciler) populateClusterMetadata(ctx context.Context, clusterNN types.NamespacedName, conn *connection) {
	if conn.initError != "" {
		return
	}

	switch conn.connType {
	case connectionTypeK8s:
		r.populateK8sMetadata(ctx, clusterNN, conn)
	case connectionTypeDocker:
		r.populateDockerMetadata(ctx, conn)
	}
}

func (r *Reconciler) populateK8sMetadata(ctx context.Context, clusterNN types.NamespacedName, conn *connection) {
	if conn.arch == "" {
		conn.arch = r.readKubernetesArch(ctx, conn.k8sClient)
	}

	if conn.registry == nil {
		reg := conn.k8sClient.LocalRegistry(ctx)
		if !container.IsEmptyRegistry(reg) {
			// If we've found a local registry in the cluster at run-time, use that
			// instead of the default_registry (if any) declared in the Tiltfile
			logger.Get(ctx).Infof("Auto-detected local registry from environment: %s", reg)

			if conn.spec.DefaultRegistry != nil {
				// The user has specified a default registry in their Tiltfile, but it will be ignored.
				logger.Get(ctx).Infof("Default registry specified, but will be ignored in favor of auto-detected registry.")
			}
		} else if conn.spec.DefaultRegistry != nil {
			logger.Get(ctx).Debugf("Using default registry from Tiltfile: %s", conn.spec.DefaultRegistry)
		} else {
			logger.Get(ctx).Debugf(
				"No local registry detected and no default registry set for cluster %q",
				clusterNN.Name)
		}

		conn.registry = reg
	}

	if conn.connStatus == nil {
		apiConfig := conn.k8sClient.APIConfig()
		k8sStatus := &v1alpha1.KubernetesClusterConnectionStatus{
			Context: apiConfig.CurrentContext,
			Product: string(k8s.ClusterProductFromAPIConfig(apiConfig)),
		}
		context, ok := apiConfig.Contexts[apiConfig.CurrentContext]
		if ok {
			k8sStatus.Namespace = context.Namespace
			k8sStatus.Cluster = context.Cluster
		}
		configPath, err := r.writeFrozenKubeConfig(ctx, clusterNN, apiConfig)
		if err != nil {
			conn.initError = err.Error()
		}
		k8sStatus.ConfigPath = configPath

		conn.connStatus = &v1alpha1.ClusterConnectionStatus{
			Kubernetes: k8sStatus,
		}
	}

	if conn.serverVersion == "" {
		versionInfo, err := conn.k8sClient.CheckConnected(ctx)
		if err == nil {
			conn.serverVersion = versionInfo.String()
		}
	}
}

func (r *Reconciler) openFrozenKubeConfigFile(ctx context.Context, nn types.NamespacedName) (string, afero.File, error) {
	path, err := r.base.RuntimeFile(
		filepath.Join(string(r.apiServerName), "cluster", fmt.Sprintf("%s.yml", nn.Name)))
	if err == nil {
		var f afero.File
		f, err = r.filesystem.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
		if err == nil {
			return path, f, nil
		}
	}

	path, err = r.base.StateFile(
		filepath.Join(string(r.apiServerName), "cluster", fmt.Sprintf("%s.yml", nn.Name)))
	if err != nil {
		return "", nil, fmt.Errorf("storing temp kubeconfigs: %v", err)
	}

	f, err := r.filesystem.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return "", nil, fmt.Errorf("storing temp kubeconfigs: %v", err)
	}
	return path, f, nil
}

func (r *Reconciler) writeFrozenKubeConfig(ctx context.Context, nn types.NamespacedName, config *api.Config) (string, error) {
	config = config.DeepCopy()
	err := api.MinifyConfig(config)
	if err != nil {
		return "", fmt.Errorf("minifying Kubernetes config: %v", err)
	}

	err = api.FlattenConfig(config)
	if err != nil {
		return "", fmt.Errorf("flattening Kubernetes config: %v", err)
	}

	obj, err := latest.Scheme.ConvertToVersion(config, latest.ExternalVersion)
	if err != nil {
		return "", fmt.Errorf("converting Kubernetes config: %v", err)
	}

	printer := printers.YAMLPrinter{}
	path, f, err := r.openFrozenKubeConfigFile(ctx, nn)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close()
	}()

	err = printer.PrintObj(obj, f)
	if err != nil {
		return "", fmt.Errorf("writing kubeconfig: %v", err)
	}
	return path, nil
}

func (r *Reconciler) populateDockerMetadata(ctx context.Context, conn *connection) {
	if conn.arch == "" {
		conn.arch = r.readDockerArch(ctx, conn.dockerClient)
	}

	if conn.serverVersion == "" {
		versionInfo, err := conn.dockerClient.ServerVersion(ctx)
		if err == nil {
			conn.serverVersion = versionInfo.Version
		}
	}
}

func (r *Reconciler) cleanup(clusterNN types.NamespacedName) {
	r.clusterHealth.Stop(clusterNN)
	r.connManager.delete(clusterNN)
}

func (c *connection) toStatus(statusErr string) v1alpha1.ClusterStatus {
	var connectedAt *metav1.MicroTime
	if c.initError == "" && !c.createdAt.IsZero() {
		t := apis.NewMicroTime(c.createdAt)
		connectedAt = &t
	}

	clusterError := c.initError
	if clusterError == "" {
		clusterError = statusErr
	}

	return v1alpha1.ClusterStatus{
		Error:       clusterError,
		Arch:        c.arch,
		Version:     c.serverVersion,
		ConnectedAt: connectedAt,
		Registry:    c.registry,
		Connection:  c.connStatus,
	}
}
