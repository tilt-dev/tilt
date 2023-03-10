package dockercomposelogstream

import (
	"context"
	"io"
	"sync"
	"time"

	dtypes "github.com/docker/docker/api/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockercomposeservices"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	client   ctrlclient.Client
	store    store.RStore
	dcc      dockercompose.DockerComposeClient
	dc       docker.Client
	requeuer *indexer.Requeuer

	mu sync.Mutex

	// Protected by the mutex.
	results         map[types.NamespacedName]*Result
	containerStates map[serviceKey]*v1alpha1.DockerContainerState
	containerIDs    map[serviceKey]string
	projectWatches  map[string]*ProjectWatch
}

var _ reconcile.Reconciler = &Reconciler{}

func NewReconciler(client ctrlclient.Client, store store.RStore,
	dcc dockercompose.DockerComposeClient, dc docker.Client) *Reconciler {
	return &Reconciler{
		client:          client,
		store:           store,
		dcc:             dcc,
		dc:              dc.ForOrchestrator(model.OrchestratorDC),
		projectWatches:  make(map[string]*ProjectWatch),
		results:         make(map[types.NamespacedName]*Result),
		containerStates: make(map[serviceKey]*v1alpha1.DockerContainerState),
		containerIDs:    make(map[serviceKey]string),
		requeuer:        indexer.NewRequeuer(),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	nn := req.NamespacedName
	obj := &v1alpha1.DockerComposeLogStream{}
	err := r.client.Get(ctx, nn, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || obj.ObjectMeta.DeletionTimestamp != nil {
		r.clearResult(nn)
		r.manageOwnedProjectWatches()
		return ctrl.Result{}, nil
	}

	ctx = store.MustObjectLogHandler(ctx, r.store, obj)
	r.manageLogWatch(ctx, nn, obj)

	// The project event streamer depends on the project we read in manageLogWatch().
	r.manageOwnedProjectWatches()

	return ctrl.Result{}, nil
}

// Removes all state for an object.
func (r *Reconciler) clearResult(nn types.NamespacedName) {
	result, ok := r.results[nn]
	if ok {
		if result.watch != nil {
			result.watch.cancel()
		}
		delete(r.results, nn)
	}
}

// Looks up the container state for the current object, if possible.
func (r *Reconciler) reconcileContainerState(ctx context.Context, obj *v1alpha1.DockerComposeLogStream, serviceKey serviceKey) {
	id, err := r.dcc.ContainerID(ctx, v1alpha1.DockerComposeServiceSpec{Project: obj.Spec.Project, Service: obj.Spec.Service})
	if err != nil {
		return
	}

	state, err := r.getContainerState(ctx, string(id))
	if err != nil {
		return
	}
	r.recordContainerState(serviceKey, string(id), state)
}

// Starts the log watcher if necessary.
func (r *Reconciler) manageLogWatch(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.DockerComposeLogStream) {
	// Make sure the result is up to date.
	result, ok := r.results[nn]
	changed := ok && !apicmp.DeepEqual(result.spec, obj.Spec)
	if changed && result.watch != nil {
		result.watch.cancel()
		result.watch = nil
	}

	if !ok {
		result = &Result{
			name:      nn,
			loggerCtx: store.MustObjectLogHandler(ctx, r.store, obj),
		}
		r.results[nn] = result
	}

	if changed || !ok {
		result.spec = obj.Spec
		result.projectHash = dockercomposeservices.MustHashProject(obj.Spec.Project)
	}

	serviceKey := result.serviceKey()
	r.reconcileContainerState(ctx, obj, serviceKey)

	containerState := r.containerStates[serviceKey]
	containerID := r.containerIDs[serviceKey]
	if containerState == nil || containerID == "" {
		return
	}

	// Docker evidently records the container start time asynchronously, so it can actually be AFTER
	// the first log timestamps (also reported by Docker), so we pad it by a second to reduce the
	// number of potentially duplicative logs
	startWatchTime := containerState.StartedAt.Time.Add(-time.Second)
	if result.watch != nil {
		if !result.watch.Done() {
			// watcher is already running
			return
		}

		if result.watch.containerID == containerID && !result.watch.startWatchTime.Before(startWatchTime) {
			// watcher finished but the container hasn't started up again
			// (N.B. we cannot compare on the container ID because containers can restart and be re-used
			// 	after being stopped for jobs that run to completion but are re-triggered)
			return
		}
	}

	if ctx.Err() != nil {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	manifestName := model.ManifestName(obj.Annotations[v1alpha1.AnnotationManifest])
	w := &watch{
		ctx:            ctx,
		cancel:         cancel,
		manifestName:   manifestName,
		nn:             nn,
		spec:           obj.Spec,
		startWatchTime: startWatchTime,
		containerID:    containerID,
	}
	result.watch = w
	go r.consumeLogs(w)
}

func (r *Reconciler) consumeLogs(watch *watch) {
	defer func() {
		watch.cancel()
		r.requeuer.Add(watch.nn)
	}()

	ctx := watch.ctx
	if ctx.Err() != nil {
		return
	}
	startTime := watch.startWatchTime

	for {
		readCloser, err := r.dc.ContainerLogs(ctx, watch.containerID, dtypes.ContainerLogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Since:      startTime.Format(time.RFC3339Nano),
		})
		if err != nil || ctx.Err() != nil {
			// container may not exist anymore, bail and let the reconciler retry.
			return
		}

		actionWriter := &LogActionWriter{
			store:        r.store,
			manifestName: watch.manifestName,
		}

		reader := runtimelog.NewHardCancelReader(ctx, readCloser)
		_, err = io.Copy(actionWriter, reader)
		_ = readCloser.Close()
		if err == nil || ctx.Err() != nil {
			// stop tailing because either:
			// 	* docker-compose logs exited naturally -> this means the container exited, so a new watcher will
			// 	  be created once a new container is seen
			//  * context was canceled -> manifest is no longer in engine & being torn-down
			return
		}

		// something went wrong with docker-compose, log it and re-attach, starting from the last
		// successfully logged timestamp
		logger.Get(watch.ctx).Debugf("Error streaming %s logs: %v", watch.nn.Name, err)
		startTime = time.Now()
	}
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DockerComposeLogStream{}).
		Watches(r.requeuer, handler.Funcs{})

	return b, nil
}

type watch struct {
	ctx            context.Context
	cancel         func()
	manifestName   model.ManifestName
	nn             types.NamespacedName
	spec           v1alpha1.DockerComposeLogStreamSpec
	startWatchTime time.Time
	containerID    string
}

func (w *watch) Done() bool {
	return w.ctx.Err() != nil
}

// Keeps track of the state we currently know about.
type Result struct {
	loggerCtx   context.Context
	name        types.NamespacedName
	projectHash string
	spec        v1alpha1.DockerComposeLogStreamSpec
	watch       *watch
}

func (r *Result) serviceKey() serviceKey {
	return serviceKey{service: r.spec.Service, projectHash: r.projectHash}
}

// Index the containers from each docker compose service.
type serviceKey struct {
	service     string
	projectHash string
}
