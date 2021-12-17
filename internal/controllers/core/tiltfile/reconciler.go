package tiltfile

import (
	"context"
	"fmt"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/restarton"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/buildcontrols"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	mu                   sync.Mutex
	st                   store.RStore
	tfl                  tiltfile.TiltfileLoader
	k8sClient            k8s.Client
	dockerClient         docker.Client
	ctrlClient           ctrlclient.Client
	k8sContextOverride   k8s.KubeContextOverride
	k8sNamespaceOverride k8s.NamespaceOverride
	indexer              *indexer.Indexer
	buildSource          *BuildSource
	engineMode           store.EngineMode
	loadCount            int // used to differentiate spans

	runs map[types.NamespacedName]*runStatus
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Tiltfile{}).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueTriggerQueue)).
		Watches(r.buildSource, handler.Funcs{})

	restarton.SetupController(b, r.indexer, func(obj ctrlclient.Object) (*v1alpha1.RestartOnSpec, *v1alpha1.StartOnSpec) {
		tf := obj.(*v1alpha1.Tiltfile)
		return tf.Spec.RestartOn, nil
	})

	return b, nil
}

func NewReconciler(st store.RStore, tfl tiltfile.TiltfileLoader, k8sClient k8s.Client, dockerClient docker.Client,
	ctrlClient ctrlclient.Client, scheme *runtime.Scheme,
	buildSource *BuildSource, engineMode store.EngineMode,
	k8sContextOverride k8s.KubeContextOverride,
	k8sNamespaceOverride k8s.NamespaceOverride) *Reconciler {
	return &Reconciler{
		st:                   st,
		tfl:                  tfl,
		k8sClient:            k8sClient,
		dockerClient:         dockerClient,
		ctrlClient:           ctrlClient,
		indexer:              indexer.NewIndexer(scheme, indexTiltfile),
		runs:                 make(map[types.NamespacedName]*runStatus),
		buildSource:          buildSource,
		engineMode:           engineMode,
		k8sContextOverride:   k8sContextOverride,
		k8sNamespaceOverride: k8sNamespaceOverride,
	}
}

// Reconcile manages Tiltfile execution.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	nn := request.NamespacedName

	var tf v1alpha1.Tiltfile
	err := r.ctrlClient.Get(ctx, nn, &tf)
	r.indexer.OnReconcile(nn, &tf)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !tf.ObjectMeta.DeletionTimestamp.IsZero() {
		r.deleteExistingRun(nn)

		// Delete owned objects
		err := updateOwnedObjects(ctx, r.ctrlClient, nn, nil, nil, r.engineMode, container.Registry{}, r.defaultK8sConnection())
		if err != nil {
			return ctrl.Result{}, err
		}
		r.st.Dispatch(tiltfiles.NewTiltfileDeleteAction(nn.Name))
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.st.Dispatch(tiltfiles.NewTiltfileUpsertAction(&tf))

	ctx = store.MustObjectLogHandler(ctx, r.st, &tf)
	run := r.runs[nn]
	if run == nil {
		// Initialize the UISession and filewatch if this has never been initialized before.
		err := updateOwnedObjects(ctx, r.ctrlClient, nn, &tf, nil, r.engineMode, container.Registry{}, r.defaultK8sConnection())
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	step := runStepNone
	if run != nil {
		step = run.step
		ctx = run.entry.WithLogger(ctx, r.st)
	}

	// If the tiltfile isn't being run, check to see if anything has triggered a run.
	if step == runStepNone || step == runStepDone {
		restartObjs, err := restarton.FetchObjects(ctx, r.ctrlClient, tf.Spec.RestartOn, nil)
		if err != nil {
			return ctrl.Result{}, err
		}

		lastRestartEventTime, _ := restarton.LastRestartEvent(tf.Spec.RestartOn, restartObjs)
		queue, err := configmap.TriggerQueue(ctx, r.ctrlClient)
		if err != nil {
			return ctrl.Result{}, err
		}

		be := r.needsBuild(ctx, nn, &tf, run, restartObjs.FileWatches, queue, lastRestartEventTime)
		if be != nil {
			r.startRunAsync(ctx, nn, &tf, be)
		}
	}

	// If the tiltfile has been loaded, we may still need to copy all its outputs
	// to the apiserver.
	if step == runStepLoaded {
		err := r.handleLoaded(ctx, nn, &tf, run.entry, run.tlr)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	run = r.runs[nn]
	if run != nil {
		newStatus := run.TiltfileStatus()
		if !apicmp.DeepEqual(newStatus, tf.Status) {
			update := tf.DeepCopy()
			update.Status = run.TiltfileStatus()
			err := r.ctrlClient.Status().Update(ctx, update)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// Modeled after BuildController.needsBuild and NextBuildReason(). Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
// 4) OR the command-line args have changed since the last Tiltfile build
// 5) OR user has manually triggered a Tiltfile build
func (r *Reconciler) needsBuild(ctx context.Context, nn types.NamespacedName, tf *v1alpha1.Tiltfile, run *runStatus, fileWatches map[string]*v1alpha1.FileWatch, triggerQueue *v1alpha1.ConfigMap, lastRestartEvent time.Time) *BuildEntry {
	var reason model.BuildReason
	filesChanged := []string{}

	step := runStepNone
	lastStartTime := time.Time{}
	lastStartArgs := []string{}
	if run != nil {
		step = run.step
		lastStartTime = run.startTime
		lastStartArgs = run.startArgs
	}

	if step == runStepNone {
		reason = reason.With(model.BuildReasonFlagInit)
	} else {
		filesChanged = restarton.FilesChanged(tf.Spec.RestartOn, fileWatches, lastStartTime)
		if len(filesChanged) > 0 {
			reason = reason.With(model.BuildReasonFlagChangedFiles)
		} else if lastRestartEvent.After(lastStartTime) {
			reason = reason.With(model.BuildReasonFlagTriggerUnknown)
		}
	}

	if !lastStartTime.IsZero() && !apicmp.DeepEqual(tf.Spec.Args, lastStartArgs) {
		reason = reason.With(model.BuildReasonFlagTiltfileArgs)
	}

	if configmap.InTriggerQueue(triggerQueue, nn) {
		reason = reason.With(configmap.TriggerQueueReason(triggerQueue, nn))
	}

	if reason == model.BuildReasonNone {
		return nil
	}

	state := r.st.RLockState()
	defer r.st.RUnlockState()

	r.loadCount++

	return &BuildEntry{
		Name:                  model.ManifestName(nn.Name),
		FilesChanged:          filesChanged,
		BuildReason:           reason,
		Args:                  tf.Spec.Args,
		TiltfilePath:          tf.Spec.Path,
		CheckpointAtExecStart: state.LogStore.Checkpoint(),
		LoadCount:             r.loadCount,
	}
}

// Start a tiltfile run asynchronously, returning immediately.
func (r *Reconciler) startRunAsync(ctx context.Context, nn types.NamespacedName, tf *v1alpha1.Tiltfile, entry *BuildEntry) {
	ctx = entry.WithLogger(ctx, r.st)
	ctx, cancel := context.WithCancel(ctx)

	r.runs[nn] = &runStatus{
		ctx:       ctx,
		cancel:    cancel,
		step:      runStepRunning,
		spec:      tf.Spec.DeepCopy(),
		entry:     entry,
		startTime: time.Now(),
		startArgs: entry.Args,
	}
	go r.run(ctx, nn, tf, entry)
}

// Executes the tiltfile on a non-blocking goroutine, and requests reconciliation on completion.
func (r *Reconciler) run(ctx context.Context, nn types.NamespacedName, tf *v1alpha1.Tiltfile, entry *BuildEntry) {
	startTime := time.Now()
	r.st.Dispatch(ConfigsReloadStartedAction{
		Name:         entry.Name,
		FilesChanged: entry.FilesChanged,
		StartTime:    startTime,
		SpanID:       SpanIDForLoadCount(entry.Name, entry.LoadCount),
		Reason:       entry.BuildReason,
	})

	buildcontrols.LogBuildEntry(ctx, buildcontrols.BuildEntry{
		Name:         entry.Name,
		BuildReason:  entry.BuildReason,
		FilesChanged: entry.FilesChanged,
	})

	if entry.BuildReason.Has(model.BuildReasonFlagTiltfileArgs) {
		logger.Get(ctx).Infof("Tiltfile args changed to: %v", entry.Args)
	}

	tlr := r.tfl.Load(ctx, tf)

	// If the user is executing an empty main tiltfile, that probably means
	// they need a tutorial. For now, we link to that tutorial, but a more interactive
	// system might make sense here.
	if tlr.Error == nil && len(tlr.Manifests) == 0 && tf.Name == model.MainTiltfileManifestName.String() {
		tlr.Error = fmt.Errorf("No resources found. Check out https://docs.tilt.dev/tutorial.html to get started!")
	}

	if tlr.Orchestrator() != model.OrchestratorUnknown {
		r.dockerClient.SetOrchestrator(tlr.Orchestrator())
	}

	if requiresDocker(tlr) {
		dockerErr := r.dockerClient.CheckConnected()
		if tlr.Error == nil && dockerErr != nil {
			tlr.Error = errors.Wrap(dockerErr, "Failed to connect to Docker")
		}
	}

	r.mu.Lock()
	run, ok := r.runs[nn]
	if ok {
		run.tlr = &tlr
		run.step = runStepLoaded
	}
	r.mu.Unlock()

	// Schedule a reconcile to create the API objects.
	r.buildSource.Add(nn)
}

// After the tiltfile has been evaluated, create all the objects in the
// apiserver.
func (r *Reconciler) handleLoaded(ctx context.Context, nn types.NamespacedName, tf *v1alpha1.Tiltfile, entry *BuildEntry, tlr *tiltfile.TiltfileLoadResult) error {
	// TODO(nick): Rewrite to handle multiple tiltfiles.
	err := updateOwnedObjects(ctx, r.ctrlClient, nn, tf, tlr, r.engineMode, r.decideRegistry(ctx, tlr), r.defaultK8sConnection())
	if err != nil {
		// If updating the API server fails, just return the error, so that the
		// reconciler will retry.
		return errors.Wrap(err, "Failed to update API server")
	}

	if tlr.Error != nil {
		logger.Get(ctx).Errorf("%s", tlr.Error.Error())
	}

	r.st.Dispatch(ConfigsReloadedAction{
		Name:                  entry.Name,
		Manifests:             tlr.Manifests,
		Tiltignore:            tlr.Tiltignore,
		ConfigFiles:           tlr.ConfigFiles,
		FinishTime:            time.Now(),
		Err:                   tlr.Error,
		Features:              tlr.FeatureFlags,
		TeamID:                tlr.TeamID,
		TelemetrySettings:     tlr.TelemetrySettings,
		Secrets:               tlr.Secrets,
		AnalyticsTiltfileOpt:  tlr.AnalyticsOpt,
		DockerPruneSettings:   tlr.DockerPruneSettings,
		CheckpointAtExecStart: entry.CheckpointAtExecStart,
		VersionSettings:       tlr.VersionSettings,
		UpdateSettings:        tlr.UpdateSettings,
		WatchSettings:         tlr.WatchSettings,
	})

	run, ok := r.runs[nn]
	if ok {
		run.step = runStepDone
		run.finishTime = time.Now()
	}

	// Schedule a reconcile in case any triggers happened while we were updating
	// API objects.
	r.buildSource.Add(nn)

	return nil
}

// Cancel execution of a running tiltfile and delete all record of it.
func (r *Reconciler) deleteExistingRun(nn types.NamespacedName) {
	run, ok := r.runs[nn]
	if !ok {
		return
	}
	delete(r.runs, nn)
	run.cancel()
}

// Find all the objects we need to watch based on the tiltfile model.
func indexTiltfile(obj client.Object) []indexer.Key {
	return nil
}

// Find any objects we need to reconcile based on the trigger queue.
func (r *Reconciler) enqueueTriggerQueue(obj client.Object) []reconcile.Request {
	cm, ok := obj.(*v1alpha1.ConfigMap)
	if !ok {
		return nil
	}

	if cm.Name != configmap.TriggerQueueName {
		return nil
	}

	// We can only trigger tiltfiles that have run once, so search
	// through the map of known tiltfiles.
	names := configmap.NamesInTriggerQueue(cm)
	r.mu.Lock()
	defer r.mu.Unlock()

	requests := []reconcile.Request{}
	for _, name := range names {
		nn := types.NamespacedName{Name: name}
		_, ok := r.runs[nn]
		if ok {
			requests = append(requests, reconcile.Request{NamespacedName: nn})
		}
	}
	return requests
}

// decideRegistry returns the image registry we should use; if detected, a pre-configured
// local registry; otherwise, the registry specified by the user via default_registry.
// Otherwise, we'll return the zero value of `s.defaultReg`, which is an empty registry.
// It has side-effects (a log line) and so should only be called once.
func (r *Reconciler) decideRegistry(ctx context.Context, tlr *tiltfile.TiltfileLoadResult) container.Registry {
	if tlr.Orchestrator() != model.OrchestratorK8s {
		return tlr.DefaultRegistry
	}

	registry := r.k8sClient.LocalRegistry(ctx)

	if !registry.Empty() {
		// If we've found a local registry in the cluster at run-time, use that
		// instead of the default_registry (if any) declared in the Tiltfile
		logger.Get(ctx).Infof("Auto-detected local registry from environment: %s", registry)
		return registry
	}

	return tlr.DefaultRegistry
}

// The kubernetes connection defined by the CLI.
func (r *Reconciler) defaultK8sConnection() *v1alpha1.KubernetesClusterConnection {
	return &v1alpha1.KubernetesClusterConnection{
		Context:   string(r.k8sContextOverride),
		Namespace: string(r.k8sNamespaceOverride),
	}
}

func requiresDocker(tlr tiltfile.TiltfileLoadResult) bool {
	if tlr.Orchestrator() == model.OrchestratorDC {
		return true
	}

	for _, m := range tlr.Manifests {
		for _, iTarget := range m.ImageTargets {
			if iTarget.IsDockerBuild() {
				return true
			}
		}
	}

	return false
}

// Represent the steps of Tiltfile execution.
type runStep int

const (
	// Tiltfile is waiting for first execution.
	runStepNone runStep = iota

	// We're currently running this tiltfile.
	runStepRunning

	// The tiltfile is loaded, but the results haven't been
	// sent to the API server.
	runStepLoaded

	// The tiltfile has created all owned objects, and may now be restarted.
	runStepDone
)

type runStatus struct {
	ctx        context.Context
	cancel     func()
	step       runStep
	spec       *v1alpha1.TiltfileSpec
	entry      *BuildEntry
	tlr        *tiltfile.TiltfileLoadResult
	startTime  time.Time
	startArgs  []string
	finishTime time.Time
}

func (rs *runStatus) TiltfileStatus() v1alpha1.TiltfileStatus {
	switch rs.step {
	case runStepRunning, runStepLoaded:
		return v1alpha1.TiltfileStatus{
			Running: &v1alpha1.TiltfileStateRunning{
				StartedAt: apis.NewMicroTime(rs.startTime),
			},
		}
	case runStepDone:
		error := ""
		if rs.tlr.Error != nil {
			error = rs.tlr.Error.Error()
		}
		return v1alpha1.TiltfileStatus{
			Terminated: &v1alpha1.TiltfileStateTerminated{
				StartedAt:  apis.NewMicroTime(rs.startTime),
				FinishedAt: apis.NewMicroTime(rs.finishTime),
				Error:      error,
			},
		}
	}

	return v1alpha1.TiltfileStatus{
		Waiting: &v1alpha1.TiltfileStateWaiting{
			Reason: "Unknown",
		},
	}
}
