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

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	mu           sync.Mutex
	st           store.RStore
	tfl          tiltfile.TiltfileLoader
	dockerClient docker.Client
	ctrlClient   ctrlclient.Client
	indexer      *indexer.Indexer
	buildSource  *BuildSource
	engineMode   store.EngineMode

	runs map[types.NamespacedName]*runStatus
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Tiltfile{}).
		Watches(&source.Kind{Type: &v1alpha1.FileWatch{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(r.buildSource, handler.Funcs{})

	return b, nil
}

func NewReconciler(st store.RStore, tfl tiltfile.TiltfileLoader, dockerClient docker.Client,
	ctrlClient ctrlclient.Client, scheme *runtime.Scheme,
	buildSource *BuildSource, engineMode store.EngineMode) *Reconciler {
	return &Reconciler{
		st:           st,
		tfl:          tfl,
		dockerClient: dockerClient,
		ctrlClient:   ctrlClient,
		indexer:      indexer.NewIndexer(scheme, indexTiltfile),
		runs:         make(map[types.NamespacedName]*runStatus),
		buildSource:  buildSource,
		engineMode:   engineMode,
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
		// Initialize the UISession if this has never been initialized before.
		err := updateOwnedObjects(ctx, r.ctrlClient, nn, &tf, nil, store.EngineModeCI)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// If the spec has changed, cancel any existing runs
	if run != nil && !apicmp.DeepEqual(run.spec, &(tf.Spec)) {
		run.cancel()
		delete(r.runs, nn)
		run = nil
	}

	step := runStepNone
	if run != nil {
		step = run.step
		ctx = run.entry.WithLogger(ctx, r.st)
	}

	// Check to see if we have a BuildEntry triggering this tiltfile.
	//
	// This is a short-term hack to make this interoperate with the legacy subscriber.
	// The "right" version would read from FileWatch and other dependencies to determine
	// when to build.
	be := r.buildSource.Entry()
	if be != nil && be.Name.String() == nn.Name && (step == runStepNone || step == runStepDone) {
		r.startRunAsync(ctx, nn, &tf, be)
	} else if step == runStepLoaded {
		err := r.handleLoaded(ctx, nn, &tf, run.entry, run.tlr)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	run = r.runs[nn]
	if run != nil {
		update := tf.DeepCopy()
		update.Status = run.TiltfileStatus()
		err := r.ctrlClient.Status().Update(ctx, update)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
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

	buildcontrol.LogBuildEntry(ctx, buildcontrol.BuildEntry{
		Name:         entry.Name,
		BuildReason:  entry.BuildReason,
		FilesChanged: entry.FilesChanged,
	})

	userConfigState := entry.UserConfigState
	if entry.BuildReason.Has(model.BuildReasonFlagTiltfileArgs) {
		logger.Get(ctx).Infof("Tiltfile args changed to: %v", userConfigState.Args)
	}

	tlr := r.tfl.Load(ctx, entry.TiltfilePath, userConfigState)
	if tlr.Error == nil && len(tlr.Manifests) == 0 {
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
	err := updateOwnedObjects(ctx, r.ctrlClient, nn, tf, tlr, r.engineMode)
	if err != nil {
		// If updating the API server fails, just return the error, so that the
		// reconciler will retry.
		return errors.Wrap(err, "Failed to update API server")
	}

	if tlr.Error != nil {
		logger.Get(ctx).Errorf("%s", tlr.Error.Error())
	}

	// We've consumed the build entry, and are ready for the next one.
	r.buildSource.SetEntry(nil)

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
		MetricsSettings:       tlr.MetricsSettings,
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
	result := []indexer.Key{}
	tiltfile := obj.(*v1alpha1.Tiltfile)
	if tiltfile.Spec.RestartOn != nil {
		fwGVK := v1alpha1.SchemeGroupVersion.WithKind("FileWatch")

		for _, name := range tiltfile.Spec.RestartOn.FileWatches {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: name},
				GVK:  fwGVK,
			})
		}
	}
	return result
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
