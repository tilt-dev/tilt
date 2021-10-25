package liveupdate

import (
	"context"
	"fmt"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

var discoveryGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesDiscovery")
var applyGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesApply")
var fwGVK = v1alpha1.SchemeGroupVersion.WithKind("FileWatch")
var imageMapGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// Manages the LiveUpdate API object.
type Reconciler struct {
	client  ctrlclient.Client
	indexer *indexer.Indexer
	store   store.RStore

	ExecUpdater   containerupdate.ContainerUpdater
	DockerUpdater containerupdate.ContainerUpdater
	updateMode    liveupdates.UpdateMode
	kubeContext   k8s.KubeContext
	startedTime   metav1.MicroTime

	monitors map[string]*monitor

	// TODO(nick): Remove this mutex once ForceApply is gone.
	mu sync.Mutex
}

var _ reconcile.Reconciler = &Reconciler{}

// Dependency-inject a live update reconciler.
func NewReconciler(
	st store.RStore,
	dcu *containerupdate.DockerUpdater,
	ecu *containerupdate.ExecUpdater,
	updateMode liveupdates.UpdateMode,
	kubeContext k8s.KubeContext,
	client ctrlclient.Client,
	scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		DockerUpdater: dcu,
		ExecUpdater:   ecu,
		updateMode:    updateMode,
		kubeContext:   kubeContext,
		client:        client,
		indexer:       indexer.NewIndexer(scheme, indexLiveUpdate),
		store:         st,
		startedTime:   apis.NowMicro(),
		monitors:      make(map[string]*monitor),
	}
}

// Create a reconciler baked by a fake ContainerUpdater and Client.
func NewFakeReconciler(
	st store.RStore,
	cu containerupdate.ContainerUpdater,
	client ctrlclient.Client) *Reconciler {
	scheme := v1alpha1.NewScheme()
	return &Reconciler{
		DockerUpdater: cu,
		ExecUpdater:   cu,
		updateMode:    liveupdates.UpdateModeAuto,
		kubeContext:   k8s.KubeContext("fake-context"),
		client:        client,
		indexer:       indexer.NewIndexer(scheme, indexLiveUpdate),
		store:         st,
		startedTime:   apis.NowMicro(),
		monitors:      make(map[string]*monitor),
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	lu := &v1alpha1.LiveUpdate{}
	err := r.client.Get(ctx, req.NamespacedName, lu)
	r.indexer.OnReconcile(req.NamespacedName, lu)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("liveupdate reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || lu.ObjectMeta.DeletionTimestamp != nil {
		r.store.Dispatch(liveupdates.NewLiveUpdateDeleteAction(req.Name))
		delete(r.monitors, req.Name)
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.store.Dispatch(liveupdates.NewLiveUpdateUpsertAction(lu))

	ctx = store.MustObjectLogHandler(ctx, r.store, lu)

	if lu.Annotations[v1alpha1.AnnotationManagedBy] != "" {
		// A LiveUpdate can't be managed by the reconciler until all the objects
		// it depends on are managed by the reconciler. The Tiltfile controller
		// is responsible for marking objects that we want to manage with ForceApply().
		return ctrl.Result{}, nil
	}

	invalidSelectorFailedState := r.ensureSelectorValid(lu)
	if invalidSelectorFailedState != nil {
		return r.handleFailure(ctx, lu, invalidSelectorFailedState)
	}

	monitor := r.ensureMonitorExists(lu.Name, lu.Spec)
	hasFileChanges, err := r.reconcileFileWatches(ctx, monitor)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleFailure(ctx, lu, createFailedState(lu, "ObjectNotFound", err.Error()))
		}
		return ctrl.Result{}, err
	}

	hasKubernetesChanges, err := r.reconcileKubernetesResource(ctx, monitor)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleFailure(ctx, lu, createFailedState(lu, "ObjectNotFound", err.Error()))
		}
		return ctrl.Result{}, err
	}

	if hasFileChanges || hasKubernetesChanges {
		monitor.hasChangesToSync = true
	}

	if monitor.hasChangesToSync {
		status := r.maybeSync(ctx, lu, monitor)
		if status.Failed != nil {
			// Log any new failures.
			isNew := lu.Status.Failed == nil || !apicmp.DeepEqual(lu.Status.Failed, status.Failed)
			if isNew {
				logger.Get(ctx).Warnf("LiveUpdate %q %s: %v", lu.Name, status.Failed.Reason, status.Failed.Message)
			}
		}

		if !apicmp.DeepEqual(lu.Status, status) {
			update := lu.DeepCopy()
			update.Status = status

			err := r.client.Status().Update(ctx, update)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	monitor.hasChangesToSync = false

	return ctrl.Result{}, nil
}

// Check for some invalid states.
func (r *Reconciler) ensureSelectorValid(lu *v1alpha1.LiveUpdate) *v1alpha1.LiveUpdateStateFailed {
	selector := lu.Spec.Selector.Kubernetes
	if selector == nil {
		return createFailedState(lu, "Invalid", "No valid selector")
	}

	if selector.DiscoveryName == "" {
		return createFailedState(lu, "Invalid", "Kubernetes selector requires DiscoveryName")
	}
	return nil
}

// If the failure state has changed, log it and write it to the apiserver.
func (r *Reconciler) handleFailure(ctx context.Context, lu *v1alpha1.LiveUpdate, failed *v1alpha1.LiveUpdateStateFailed) (ctrl.Result, error) {
	isNew := lu.Status.Failed == nil || !apicmp.DeepEqual(lu.Status.Failed, failed)
	if !isNew {
		return ctrl.Result{}, nil
	}

	logger.Get(ctx).Warnf("LiveUpdate %q %s: %v", lu.Name, failed.Reason, failed.Message)

	update := lu.DeepCopy()
	update.Status.Failed = failed

	err := r.client.Status().Update(ctx, update)

	return ctrl.Result{}, err
}

// Create the monitor that tracks a live update. If the live update
// spec changes, wipe out all accumulated state.
func (r *Reconciler) ensureMonitorExists(name string, spec v1alpha1.LiveUpdateSpec) *monitor {
	m, ok := r.monitors[name]
	if ok && apicmp.DeepEqual(spec, m.spec) {
		return m
	}

	m = &monitor{
		spec:           spec,
		modTimeByPath:  make(map[string]metav1.MicroTime),
		lastFileEvents: make(map[string]*v1alpha1.FileEvent),
		containers:     make(map[monitorContainerKey]monitorContainerStatus),
	}
	r.monitors[name] = m
	return m
}

// Consume all FileEvents off the FileWatch objects.
// Returns true if we saw new file events.
//
// TODO(nick): Currently, it's entirely possible to miss file events.  This has
// always been true (since operating systems themselves put limits on the event
// queue.) But it gets worse in a world where we read FileEvents from the API,
// since the FileWatch API itself adds lower limits.
//
// Long-term, we ought to have some way to reconnect/resync like other
// sync systems do (syncthing/rsync). e.g., diff the two file systems
// and update based on changes. But it also might make more sense to switch to a
// different library for syncing (e.g., Mutagen) now that live updates
// are decoupled from other file event-triggered tasks.
//
// In the meantime, Milas+Nick should figure out a way to handle this
// better in the short term.
func (r *Reconciler) reconcileFileWatches(ctx context.Context, monitor *monitor) (bool, error) {
	if len(monitor.spec.FileWatchNames) == 0 {
		return false, nil
	}

	hasChange := false
	for _, fwn := range monitor.spec.FileWatchNames {
		oneChange, err := r.reconcileOneFileWatch(ctx, monitor, fwn)
		if err != nil {
			return false, err
		}
		if oneChange {
			hasChange = true
		}
	}
	return hasChange, nil
}

// Consume one FileWatch object.
func (r *Reconciler) reconcileOneFileWatch(ctx context.Context, monitor *monitor, fwn string) (bool, error) {
	var fw v1alpha1.FileWatch
	err := r.client.Get(ctx, types.NamespacedName{Name: fwn}, &fw)
	if err != nil {
		return false, err
	}

	events := fw.Status.FileEvents
	if len(events) == 0 {
		return false, nil
	}

	newLastFileEvent := events[len(events)-1]
	event := monitor.lastFileEvents[fwn]
	if event != nil && apicmp.DeepEqual(&newLastFileEvent, event) {
		return false, nil
	}
	monitor.lastFileEvents[fwn] = &newLastFileEvent

	// Consume all the file events.
	for _, event := range events {
		for _, f := range event.SeenFiles {
			existing, ok := monitor.modTimeByPath[f]
			if !ok || existing.Time.Before(event.Time.Time) {
				monitor.modTimeByPath[f] = event.Time
			}
		}
	}
	return true, nil
}

// Consume all objects off the KubernetesSelector.
// Returns true if we saw any changes to the objects we're watching.
func (r *Reconciler) reconcileKubernetesResource(ctx context.Context, monitor *monitor) (bool, error) {
	selector := monitor.spec.Selector.Kubernetes
	if selector == nil {
		return false, nil
	}

	var kd *v1alpha1.KubernetesDiscovery
	var ka *v1alpha1.KubernetesApply
	var im *v1alpha1.ImageMap
	changed := false
	if selector.ApplyName != "" {
		ka = &v1alpha1.KubernetesApply{}
		err := r.client.Get(ctx, types.NamespacedName{Name: selector.ApplyName}, ka)
		if err != nil {
			return false, err
		}

		if monitor.lastKubernetesApplyStatus == nil ||
			!apicmp.DeepEqual(monitor.lastKubernetesApplyStatus, &(ka.Status)) {
			changed = true
		}
	}

	kd = &v1alpha1.KubernetesDiscovery{}
	err := r.client.Get(ctx, types.NamespacedName{Name: selector.DiscoveryName}, kd)
	if err != nil {
		return false, err
	}

	if monitor.lastKubernetesDiscovery == nil ||
		!apicmp.DeepEqual(monitor.lastKubernetesDiscovery.Status, kd.Status) {
		changed = true
	}

	if selector.ImageMapName != "" {
		im = &v1alpha1.ImageMap{}
		err := r.client.Get(ctx, types.NamespacedName{Name: selector.ImageMapName}, im)
		if err != nil {
			return false, err
		}

		if monitor.lastImageMapStatus == nil ||
			!apicmp.DeepEqual(monitor.lastImageMapStatus, &(im.Status)) {
			changed = true
		}
	}

	if im == nil {
		monitor.lastImageMapStatus = nil
	} else {
		monitor.lastImageMapStatus = &(im.Status)
	}

	if ka == nil {
		monitor.lastKubernetesApplyStatus = nil
	} else {
		monitor.lastKubernetesApplyStatus = &(ka.Status)
	}

	monitor.lastKubernetesDiscovery = kd

	return changed, nil
}

// Convert the currently tracked state into a set of inputs
// to the updater, then apply them.
func (r *Reconciler) maybeSync(ctx context.Context, lu *v1alpha1.LiveUpdate, monitor *monitor) v1alpha1.LiveUpdateStatus {
	var status v1alpha1.LiveUpdateStatus
	kSelector := lu.Spec.Selector.Kubernetes
	if kSelector == nil {
		status.Failed = createFailedState(lu, "Invalid", "no valid selector")
		return status
	}

	kResource, err := k8sconv.NewKubernetesResource(monitor.lastKubernetesDiscovery, monitor.lastKubernetesApplyStatus)
	if err != nil {
		status.Failed = createFailedState(lu, "KubernetesError", fmt.Sprintf("creating kube resource: %v", err))
		return status
	}

	// TODO(nick): Now that we have multiple monitors, there's no reason
	// why we can't update multiple pods.
	cInfos, err := liveupdates.RunningContainersForOnePod(kSelector, kResource)
	if err != nil {
		status.Failed = createFailedState(lu, "KubernetesError", fmt.Sprintf("determining containers: %v", err))
		return status
	}

	// TODO(nick): LiveUpdateStatus needs to distinguish between two different cases:
	// 1) We can't live update the pod because it's still being scheduled/initialized.
	// 2) We can't live update the pod because it's failed, and will never recover.
	//
	// In the first case, we need to wait. In the second case, we need to tell
	// the rest of tilt to rebuild. This logic currently live in buildcontrol
	// and should be moved here.
	if len(cInfos) == 0 {
		return status
	}

	for _, c := range cInfos {
		highWaterMark := r.startedTime
		if !monitor.lastImageMapStatus.BuildStartTime.IsZero() {
			highWaterMark = *monitor.lastImageMapStatus.BuildStartTime
		}

		cKey := monitorContainerKey{
			containerID: c.ContainerID.String(),
			podName:     c.PodID.String(),
			namespace:   c.Namespace.String(),
		}
		cStatus, ok := monitor.containers[cKey]
		if ok {
			if !cStatus.lastFileTimeSynced.IsZero() {
				highWaterMark = cStatus.lastFileTimeSynced
			}

			if cStatus.failedReason != "" {
				status.Failed = createFailedState(lu, cStatus.failedReason, cStatus.failedMessage)
				return status
			}
		}

		// Determine the changed files.
		filesChanged := []string{}
		newHighWaterMark := highWaterMark
		for f, t := range monitor.modTimeByPath {
			if t.After(highWaterMark.Time) {
				filesChanged = append(filesChanged, f)

				if t.After(newHighWaterMark.Time) {
					newHighWaterMark = t
				}
			}
		}

		// Sort the files so that they're deterministic.
		sort.Slice(filesChanged, func(i, j int) bool {
			return filesChanged[i] < filesChanged[j]
		})

		oneUpdateStatus := r.applyLiveUpdatePlan(ctx, lu.Spec, c, filesChanged, newHighWaterMark, cStatus)
		adjustFailedStateTimestamps(lu, &oneUpdateStatus)

		// Update the monitor based on the result of the applied changes.
		if oneUpdateStatus.Failed != nil {
			cStatus.failedReason = oneUpdateStatus.Failed.Reason
			cStatus.failedMessage = oneUpdateStatus.Failed.Message
		} else {
			cStatus.lastFileTimeSynced = newHighWaterMark
		}
		monitor.containers[cKey] = cStatus

		// Update the status based on the result of the applied changes.
		if oneUpdateStatus.Failed != nil {
			status.Failed = oneUpdateStatus.Failed
			return status
		}

		status.Containers = append(status.Containers, oneUpdateStatus.Containers...)
	}
	return status
}

func (r *Reconciler) applyLiveUpdatePlan(
	ctx context.Context,
	spec v1alpha1.LiveUpdateSpec,
	c liveupdates.Container,
	filesChanged []string,
	newHighWaterMark metav1.MicroTime,
	lastStatus monitorContainerStatus) v1alpha1.LiveUpdateStatus {

	var status v1alpha1.LiveUpdateStatus
	plan, err := liveupdates.NewLiveUpdatePlan(spec, filesChanged)
	if err != nil {
		status.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason:  "UpdateStopped",
			Message: fmt.Sprintf("No update plan: %v", err),
		}
		return status
	}

	if len(plan.NoMatchPaths) > 0 {
		status.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason: "UpdateStopped",
			Message: fmt.Sprintf("Found file(s) not matching any sync (files: %s)",
				ospath.FormatFileChangeList(plan.NoMatchPaths)),
		}
		return status
	}

	// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
	if len(plan.StopPaths) != 0 {
		status.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason:  "UpdateStopped",
			Message: fmt.Sprintf("Detected change to stop file %q", plan.StopPaths[0]),
		}
		return status
	}

	if len(plan.SyncPaths) == 0 {
		// No files matched a sync for this image, no Live Update to run
		status.Containers = []v1alpha1.LiveUpdateContainerStatus{{
			ContainerName:      c.ContainerName.String(),
			ContainerID:        c.ContainerID.String(),
			PodName:            c.PodID.String(),
			Namespace:          c.Namespace.String(),
			LastFileTimeSynced: lastStatus.lastFileTimeSynced,
		}}
		return status
	}

	// Apply the changes to the cluster.
	input := Input{
		IsDC:               false, // update this once we support DockerCompose in the API.
		ChangedFiles:       plan.SyncPaths,
		Containers:         []liveupdates.Container{c},
		LastFileTimeSynced: newHighWaterMark,
	}
	return r.applyInternal(ctx, spec, input)
}

// Generate the correct transition time on the Failed state.
func adjustFailedStateTimestamps(obj *v1alpha1.LiveUpdate, newStatus *v1alpha1.LiveUpdateStatus) {
	if newStatus.Failed == nil {
		return
	}

	newStatus.Failed = createFailedState(obj, newStatus.Failed.Reason, newStatus.Failed.Message)
}

// Create a new failed state and update the transition timestamp if appropriate.
func createFailedState(obj *v1alpha1.LiveUpdate, reason, msg string) *v1alpha1.LiveUpdateStateFailed {
	failed := &v1alpha1.LiveUpdateStateFailed{Reason: reason, Message: msg}
	transitionTime := apis.NowMicro()
	if obj.Status.Failed != nil && obj.Status.Failed.Reason == failed.Reason {
		// If the reason hasn't changed, don't treat this as a transition.
		transitionTime = obj.Status.Failed.LastTransitionTime
	}

	failed.LastTransitionTime = transitionTime
	return failed
}

// Live-update containers by copying files and running exec commands.
//
// Update the apiserver when finished.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// decides when to kick off the live update, and run a full image build+deploy if it
// fails. Eventually we'll invert that relationship, so that BuildController
// (and other API reconcilers) watch the live update API.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.LiveUpdateSpec,
	input Input) (v1alpha1.LiveUpdateStatus, error) {
	var obj v1alpha1.LiveUpdate
	err := r.client.Get(ctx, nn, &obj)
	if err != nil {
		return v1alpha1.LiveUpdateStatus{}, err
	}

	status := r.applyInternal(ctx, spec, input)
	adjustFailedStateTimestamps(&obj, &status)

	if !apicmp.DeepEqual(status, obj.Status) {
		update := obj.DeepCopy()
		update.Status = status
		err := r.client.Status().Update(ctx, update)
		if err != nil {
			return v1alpha1.LiveUpdateStatus{}, err
		}
	}

	return status, nil
}

// Like apply, but doesn't write the status to the apiserver.
func (r *Reconciler) applyInternal(
	ctx context.Context,
	spec v1alpha1.LiveUpdateSpec,
	input Input) v1alpha1.LiveUpdateStatus {

	var result v1alpha1.LiveUpdateStatus
	cu := r.containerUpdater(input)
	l := logger.Get(ctx)
	containers := input.Containers
	cIDStr := container.ShortStrs(liveupdates.IDsForContainers(containers))
	suffix := ""
	if len(containers) != 1 {
		suffix = "(s)"
	}

	runSteps := liveupdate.RunSteps(spec)
	changedFiles := input.ChangedFiles
	hotReload := !liveupdate.ShouldRestart(spec)
	boiledSteps, err := build.BoilRuns(runSteps, changedFiles)
	if err != nil {
		result.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason:  "Invalid",
			Message: fmt.Sprintf("Building exec: %v", err),
		}
		return result
	}

	// rm files from container
	toRemove, toArchive, err := build.MissingLocalPaths(ctx, changedFiles)
	if err != nil {
		result.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason:  "Invalid",
			Message: fmt.Sprintf("Mapping paths: %v", err),
		}
		return result
	}

	if len(toRemove) > 0 {
		l.Infof("Will delete %d file(s) from container%s: %s", len(toRemove), suffix, cIDStr)
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container%s: %s", len(toArchive), suffix, cIDStr)
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	var lastExecErrorStatus *v1alpha1.LiveUpdateContainerStatus
	for _, cInfo := range containers {
		archive := build.TarArchiveForPaths(ctx, toArchive, nil)
		err = cu.UpdateContainer(ctx, cInfo, archive,
			build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)

		lastFileTimeSynced := input.LastFileTimeSynced
		if lastFileTimeSynced.IsZero() {
			lastFileTimeSynced = apis.NowMicro()
		}

		cStatus := v1alpha1.LiveUpdateContainerStatus{
			ContainerName:      cInfo.ContainerName.String(),
			ContainerID:        cInfo.ContainerID.String(),
			PodName:            cInfo.PodID.String(),
			Namespace:          cInfo.Namespace.String(),
			LastFileTimeSynced: lastFileTimeSynced,
		}

		if err != nil {
			if runFail, ok := build.MaybeRunStepFailure(err); ok {
				// Keep running updates -- we want all containers to have the same files on them
				// even if the Runs don't succeed
				logger.Get(ctx).Infof("  → Failed to update container %s: run step %q failed with exit code: %d",
					cInfo.ContainerID.ShortStr(), runFail.Cmd.String(), runFail.ExitCode)
				cStatus.LastExecError = err.Error()
				lastExecErrorStatus = &cStatus
			} else {
				// Something went wrong with this update and it's NOT the user's fault--
				// likely a infrastructure error. Bail, and fall back to full build.
				result.Failed = &v1alpha1.LiveUpdateStateFailed{
					Reason:  "UpdateFailed",
					Message: fmt.Sprintf("Updating pod %s: %v", cStatus.PodName, err),
				}
				return result
			}
		} else {
			logger.Get(ctx).Infof("  → Container %s updated!", cInfo.ContainerID.ShortStr())
			if lastExecErrorStatus != nil {
				// This build succeeded, but previously at least one failed due to user error.
				// We may have inconsistent state--bail, and fall back to full build.
				result.Failed = &v1alpha1.LiveUpdateStateFailed{
					Reason: "PodsInconsistent",
					Message: fmt.Sprintf("Pods in inconsistent state. Success: pod %s. Failure: pod %s. Error: %v",
						cStatus.PodName, lastExecErrorStatus.PodName, lastExecErrorStatus.LastExecError),
				}
				return result
			}
		}

		result.Containers = append(result.Containers, cStatus)
	}
	return result
}

func (r *Reconciler) containerUpdater(input Input) containerupdate.ContainerUpdater {
	isDC := input.IsDC
	if isDC || r.updateMode == liveupdates.UpdateModeContainer {
		return r.DockerUpdater
	}

	if r.updateMode == liveupdates.UpdateModeKubectlExec {
		return r.ExecUpdater
	}

	dcu, ok := r.DockerUpdater.(*containerupdate.DockerUpdater)
	if ok && dcu.WillBuildToKubeContext(r.kubeContext) {
		return r.DockerUpdater
	}

	return r.ExecUpdater
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.LiveUpdate{}).
		Watches(&source.Kind{Type: &v1alpha1.KubernetesDiscovery{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.KubernetesApply{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.FileWatch{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.ImageMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

// indexLiveUpdate returns keys of objects referenced _by_ the LiveUpdate object for reverse lookup including:
//  - FileWatch
//  - ImageMapName
// 	- KubernetesDiscovery
//	- KubernetesApply
func indexLiveUpdate(obj ctrlclient.Object) []indexer.Key {
	lu := obj.(*v1alpha1.LiveUpdate)
	var result []indexer.Key

	for _, fwn := range lu.Spec.FileWatchNames {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{
				Namespace: lu.Namespace,
				Name:      fwn,
			},
			GVK: fwGVK,
		})
	}

	if lu.Spec.Selector.Kubernetes != nil {
		if lu.Spec.Selector.Kubernetes.DiscoveryName != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      lu.Spec.Selector.Kubernetes.DiscoveryName,
				},
				GVK: discoveryGVK,
			})
		}

		if lu.Spec.Selector.Kubernetes.ApplyName != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      lu.Spec.Selector.Kubernetes.ApplyName,
				},
				GVK: applyGVK,
			})
		}

		if lu.Spec.Selector.Kubernetes.ImageMapName != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      lu.Spec.Selector.Kubernetes.ImageMapName,
				},
				GVK: imageMapGVK,
			})
		}
	}
	return result
}
