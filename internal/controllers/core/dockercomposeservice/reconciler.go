package dockercomposeservice

import (
	"context"
	"strings"
	"sync"

	dtypes "github.com/docker/docker/api/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/docker/go-connections/nat"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/imagemap"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/filteredwriter"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockercomposeservices"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	dcc          dockercompose.DockerComposeClient
	dc           docker.Client
	st           store.RStore
	ctrlClient   ctrlclient.Client
	indexer      *indexer.Indexer
	requeuer     *indexer.Requeuer
	disableQueue *DisableSubscriber
	mu           sync.Mutex

	// Protected by the mutex.
	results                        map[types.NamespacedName]*Result
	resultsByServiceName           map[string]*Result
	healthcheckOutputByServiceName map[string]string
	projectWatches                 map[string]*ProjectWatch
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.DockerComposeService{}).
		WatchesRawSource(r.requeuer).
		Watches(&v1alpha1.ImageMap{},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&v1alpha1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func NewReconciler(
	ctrlClient ctrlclient.Client,
	dcc dockercompose.DockerComposeClient,
	dc docker.Client,
	st store.RStore,
	scheme *runtime.Scheme,
	disableQueue *DisableSubscriber,
) *Reconciler {
	return &Reconciler{
		ctrlClient:                     ctrlClient,
		dcc:                            dcc,
		dc:                             dc.ForOrchestrator(model.OrchestratorDC),
		indexer:                        indexer.NewIndexer(scheme, indexDockerComposeService),
		st:                             st,
		requeuer:                       indexer.NewRequeuer(),
		disableQueue:                   disableQueue,
		results:                        make(map[types.NamespacedName]*Result),
		resultsByServiceName:           make(map[string]*Result),
		healthcheckOutputByServiceName: make(map[string]string),
		projectWatches:                 make(map[string]*ProjectWatch),
	}
}

// Redeploy the docker compose service when its spec
// changes or any of its dependencies change.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var obj v1alpha1.DockerComposeService
	err := r.ctrlClient.Get(ctx, nn, &obj)
	r.indexer.OnReconcile(nn, &obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !obj.ObjectMeta.DeletionTimestamp.IsZero() {
		rs, ok := r.updateForDisableQueue(nn, true /* deleting */)
		if ok {
			r.disableQueue.UpdateQueue(rs)
		}
		r.clearResult(nn)

		r.st.Dispatch(dockercomposeservices.NewDockerComposeServiceDeleteAction(nn.Name))
		r.manageOwnedProjectWatches(ctx)
		return r.manageOwnedLogStream(ctx, nn, nil)
	}

	r.st.Dispatch(dockercomposeservices.NewDockerComposeServiceUpsertAction(&obj))

	// Get configmap's disable status
	ctx = store.MustObjectLogHandler(ctx, r.st, &obj)
	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, r.ctrlClient, obj.Spec.DisableSource, obj.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.recordSpecAndDisableStatus(nn, obj.Spec, *disableStatus)

	rs, ok := r.updateForDisableQueue(nn, disableStatus.State == v1alpha1.DisableStateDisabled)
	if ok {
		r.disableQueue.UpdateQueue(rs)
		if disableStatus.State == v1alpha1.DisableStateDisabled {
			r.recordRmOnDisable(nn)
		}
	}

	if disableStatus.State != v1alpha1.DisableStateDisabled {
		// Fetch all the images needed to apply this YAML.
		imageMaps, err := imagemap.NamesToObjects(ctx, r.ctrlClient, obj.Spec.ImageMaps)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Apply to the cluster if necessary.
		if r.shouldDeployOnReconcile(request.NamespacedName, &obj, imageMaps) {
			// If we have no image dependencies in tilt, tell docker compose
			// to handle any necessary image builds.
			dcManagedBuild := len(imageMaps) == 0
			_ = r.forceApplyHelper(ctx, nn, obj.Spec, imageMaps, dcManagedBuild)
		}
	}

	// TODO(nick): Deploy dockercompose services that aren't managed via buildcontrol

	err = r.maybeUpdateStatus(ctx, nn, &obj)
	if err != nil {
		return ctrl.Result{}, err
	}
	r.manageOwnedProjectWatches(ctx)
	return r.manageOwnedLogStream(ctx, nn, &obj)
}

// Determine if we should deploy the current YAML.
//
// Ensures:
//  1. We have enough info to deploy, and
//  2. Either we haven't deployed before,
//     or one of the inputs has changed since the last deploy.
func (r *Reconciler) shouldDeployOnReconcile(
	nn types.NamespacedName,
	obj *v1alpha1.DockerComposeService,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
) bool {
	if obj.Annotations[v1alpha1.AnnotationManagedBy] != "" {
		// Until resource dependencies are expressed in the API,
		// we can't use reconciliation to deploy KubernetesApply objects
		// managed by the buildcontrol engine.
		return false
	}

	for _, imageMapName := range obj.Spec.ImageMaps {
		_, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			// We haven't built the images yet to deploy.
			return false
		}
	}

	r.mu.Lock()
	result, ok := r.results[nn]
	r.mu.Unlock()

	if !ok || result.Status.LastApplyStartTime.IsZero() {
		// We've never successfully deployed before, so deploy now.
		return true
	}

	if !apicmp.DeepEqual(obj.Spec, result.Spec) {
		// The YAML to deploy changed.
		return true
	}

	imageMapNames := obj.Spec.ImageMaps
	if len(imageMapNames) != len(result.ImageMapSpecs) ||
		len(imageMapNames) != len(result.ImageMapStatuses) {
		return true
	}

	for i, name := range obj.Spec.ImageMaps {
		im := imageMaps[types.NamespacedName{Name: name}]
		if !apicmp.DeepEqual(im.Spec, result.ImageMapSpecs[i]) {

			return true
		}
		if !apicmp.DeepEqual(im.Status, result.ImageMapStatuses[i]) {
			return true
		}
	}

	return false
}

// We need to update the disable queue in two cases:
// 1) If the resource is enabled (to clear any pending deletes), or
// 2) If the resource is deleted but still running (to kickoff a delete).
func (r *Reconciler) updateForDisableQueue(nn types.NamespacedName, isDisabled bool) (resourceState, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, isExisting := r.results[nn]
	if !isExisting {
		return resourceState{}, false
	}

	if !isDisabled {
		return resourceState{Name: nn.Name, Spec: result.Spec}, true
	}

	// We only need to do cleanup if there's a container available.
	if result.Status.ContainerState != nil {
		return resourceState{
			Name:         nn.Name,
			Spec:         result.Spec,
			NeedsCleanup: true,
			StartTime:    result.Status.ContainerState.StartedAt.Time,
		}, true
	}

	return resourceState{}, false
}

// Records that a delete was performed.
func (r *Reconciler) recordRmOnDisable(nn types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, isExisting := r.results[nn]
	if !isExisting {
		return
	}

	result.Status.ContainerID = ""
	result.Status.ContainerName = ""
	result.Status.ContainerState = nil
	result.Status.PortBindings = nil
}

// Removes all state for an object.
func (r *Reconciler) clearResult(nn types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result, ok := r.results[nn]
	if ok {
		delete(r.resultsByServiceName, result.Spec.Service)
		delete(r.results, nn)
	}
}

// Create a result object if necessary. Caller must hold the mutex.
func (r *Reconciler) ensureResultExists(nn types.NamespacedName) *Result {
	existing, hasExisting := r.results[nn]
	if hasExisting {
		return existing
	}

	result := &Result{Name: nn}
	r.results[nn] = result
	return result
}

// Record disable state of the service.
func (r *Reconciler) recordSpecAndDisableStatus(
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	disableStatus v1alpha1.DisableStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)
	if !apicmp.DeepEqual(result.Spec, spec) {
		delete(r.resultsByServiceName, result.Spec.Service)
		result.Spec = spec
		result.ProjectHash = dockercomposeservices.MustHashProject(spec.Project)
		r.resultsByServiceName[result.Spec.Service] = result
	}

	if apicmp.DeepEqual(result.Status.DisableStatus, &disableStatus) {
		return
	}

	update := result.Status.DeepCopy()
	update.DisableStatus = &disableStatus
	result.Status = *update
}

// A helper that deletes the Docker Compose service, even if they haven't been applied yet.
//
// Primarily intended so that the build controller can do force restarts.
func (r *Reconciler) ForceDelete(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	reason string) error {
	out := logger.Get(ctx).Writer(logger.InfoLvl)
	out = filteredwriter.New(out, func(s string) bool {
		// https://app.shortcut.com/windmill/story/13147/docker-compose-down-messages-for-disabled-resources-may-be-confusing
		return strings.HasPrefix(s, "Going to remove")
	})
	err := r.dcc.Rm(ctx, []v1alpha1.DockerComposeServiceSpec{spec}, out, out)
	if err != nil {
		logger.Get(ctx).Errorf("Error %s: %v", reason, err)
	}
	r.clearResult(nn)
	r.requeuer.Add(nn)
	return nil
}

// Apply the DockerCompose service spec, unconditionally,
// and requeue the reconciler so that it updates the apiserver.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// handles dependencies between resources. The API server doesn't know about build
// dependencies yet. So Tiltfile-owned resources are applied manually, rather than
// going through the normal reconcile system.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	dcManagedBuild bool) v1alpha1.DockerComposeServiceStatus {
	status := r.forceApplyHelper(ctx, nn, spec, imageMaps, dcManagedBuild)
	r.requeuer.Add(nn)
	return status
}

// Records status when an apply fail.
// This might mean the image build failed, if we're using dc-managed image builds.
// Does not necessarily clear the current running container.
func (r *Reconciler) recordApplyError(
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	err error,
	startTime metav1.MicroTime,
) v1alpha1.DockerComposeServiceStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)
	status := result.Status.DeepCopy()
	status.LastApplyStartTime = startTime
	status.LastApplyFinishTime = apis.NowMicro()
	status.ApplyError = err.Error()
	result.Status = *status
	result.SetImageMapInputs(spec, imageMaps)
	return *status
}

// Records status when an apply succeeds.
func (r *Reconciler) recordApplyStatus(
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	newStatus v1alpha1.DockerComposeServiceStatus,
) v1alpha1.DockerComposeServiceStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)
	disableStatus := result.Status.DisableStatus
	newStatus.DisableStatus = disableStatus
	result.Status = newStatus
	result.SetImageMapInputs(spec, imageMaps)

	return newStatus
}

// A helper that applies the given specs to the cluster,
// tracking the state of the deploy in the results map.
func (r *Reconciler) forceApplyHelper(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.DockerComposeServiceSpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	// TODO(nick): Figure out a better way to infer the dcManagedBuild setting.
	dcManagedBuild bool,
) v1alpha1.DockerComposeServiceStatus {
	startTime := apis.NowMicro()
	stdout := logger.Get(ctx).Writer(logger.InfoLvl)
	stderr := logger.Get(ctx).Writer(logger.InfoLvl)
	err := r.dcc.Up(ctx, spec, dcManagedBuild, stdout, stderr)
	if err != nil {
		return r.recordApplyError(nn, spec, imageMaps, err, startTime)
	}

	// grab the initial container state
	cid, err := r.dcc.ContainerID(ctx, spec)
	if err != nil {
		return r.recordApplyError(nn, spec, imageMaps, err, startTime)
	}

	containerJSON, err := r.dc.ContainerInspect(ctx, string(cid))
	if err != nil {
		logger.Get(ctx).Debugf("Error inspecting container %s: %v", cid, err)
	}

	name := ""
	var containerState *dtypes.ContainerState
	if containerJSON.ContainerJSONBase != nil && containerJSON.ContainerJSONBase.State != nil {
		containerState = containerJSON.ContainerJSONBase.State

		// NOTE(nick): For some reason, docker container names start with "/"
		// but are printed to the user without it.
		name = strings.TrimPrefix(containerJSON.ContainerJSONBase.Name, "/")
	}

	var ports nat.PortMap
	if containerJSON.NetworkSettings != nil {
		ports = containerJSON.NetworkSettings.NetworkSettingsBase.Ports
	}

	status := dockercompose.ToServiceStatus(cid, name, containerState, ports)
	status.LastApplyStartTime = startTime
	status.LastApplyFinishTime = apis.NowMicro()
	return r.recordApplyStatus(nn, spec, imageMaps, status)
}

// Update the status on the apiserver if necessary.
func (r *Reconciler) maybeUpdateStatus(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.DockerComposeService) error {
	newStatus := v1alpha1.DockerComposeServiceStatus{}
	existing, ok := r.results[nn]
	if ok {
		newStatus = existing.Status
	}

	if apicmp.DeepEqual(obj.Status, newStatus) {
		return nil
	}

	oldError := obj.Status.ApplyError
	newError := newStatus.ApplyError
	update := obj.DeepCopy()
	update.Status = *(newStatus.DeepCopy())

	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return err
	}

	// Print new errors on objects that aren't managed by the buildcontroller.
	if newError != "" && oldError != newError && update.Annotations[v1alpha1.AnnotationManagedBy] == "" {
		logger.Get(ctx).Errorf("dockercomposeservice %s: %s", obj.Name, newError)
	}
	return nil
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// indexDockerComposeService returns keys for all the objects we need to watch based on the spec.
func indexDockerComposeService(obj client.Object) []indexer.Key {
	dcs := obj.(*v1alpha1.DockerComposeService)
	result := []indexer.Key{}
	for _, name := range dcs.Spec.ImageMaps {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: name},
			GVK:  imGVK,
		})
	}

	if dcs.Spec.DisableSource != nil {
		cm := dcs.Spec.DisableSource.ConfigMap
		if cm != nil {
			cmGVK := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: cm.Name},
				GVK:  cmGVK,
			})
		}
	}

	return result
}

// Keeps track of the state we currently know about.
type Result struct {
	Name             types.NamespacedName
	Spec             v1alpha1.DockerComposeServiceSpec
	ImageMapSpecs    []v1alpha1.ImageMapSpec
	ImageMapStatuses []v1alpha1.ImageMapStatus
	ProjectHash      string

	Status v1alpha1.DockerComposeServiceStatus
}

func (r *Result) SetImageMapInputs(spec v1alpha1.DockerComposeServiceSpec, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) {
	r.ImageMapSpecs = nil
	r.ImageMapStatuses = nil
	for _, imageMapName := range spec.ImageMaps {
		im, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			// this should never happen, but if it does, just continue quietly.
			continue
		}

		r.ImageMapSpecs = append(r.ImageMapSpecs, im.Spec)
		r.ImageMapStatuses = append(r.ImageMapStatuses, im.Status)
	}
}

// Keeps track of the projects we're currently watching.
type ProjectWatch struct {
	ctx     context.Context
	cancel  func()
	project v1alpha1.DockerComposeProject
	hash    string
}
