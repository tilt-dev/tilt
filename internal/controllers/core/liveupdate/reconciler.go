package liveupdate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	v1 "k8s.io/api/core/v1"
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
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/containerupdate"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/buildcontrols"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/store/liveupdates"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

const (
	RunStepFailure = "RunStepFailure"
)

var discoveryGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesDiscovery")
var applyGVK = v1alpha1.SchemeGroupVersion.WithKind("KubernetesApply")
var fwGVK = v1alpha1.SchemeGroupVersion.WithKind("FileWatch")
var imageMapGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

var reasonObjectNotFound = "ObjectNotFound"

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

	monitor := r.ensureMonitorExists(lu.Name, lu)
	hasFileChanges, err := r.reconcileSources(ctx, monitor)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleFailure(ctx, lu, createFailedState(lu, reasonObjectNotFound, err.Error()))
		}
		return ctrl.Result{}, err
	}

	hasKubernetesChanges, err := r.reconcileKubernetesResource(ctx, monitor)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.handleFailure(ctx, lu, createFailedState(lu, reasonObjectNotFound, err.Error()))
		}
		return ctrl.Result{}, err
	}

	hasTriggerQueueChanges, err := r.reconcileTriggerQueue(ctx, monitor)
	if err != nil {
		return ctrl.Result{}, err
	}

	if hasFileChanges || hasKubernetesChanges || hasTriggerQueueChanges {
		monitor.hasChangesToSync = true
	}

	if monitor.hasChangesToSync {
		status := r.maybeSync(ctx, lu, monitor)
		if status.Failed != nil {
			// Log any new failures.
			isNew := lu.Status.Failed == nil || !apicmp.DeepEqual(lu.Status.Failed, status.Failed)
			if isNew && r.shouldLogFailureReason(status.Failed) {
				logger.Get(ctx).Infof("LiveUpdate %q %s: %v", lu.Name, status.Failed.Reason, status.Failed.Message)
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

func (r *Reconciler) shouldLogFailureReason(obj *v1alpha1.LiveUpdateStateFailed) bool {
	// ObjectNotFound errors are normal before the Apply has created the KubernetesDiscovery object.
	return obj.Reason != reasonObjectNotFound
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

	if r.shouldLogFailureReason(failed) {
		logger.Get(ctx).Infof("LiveUpdate %q %s: %v", lu.Name, failed.Reason, failed.Message)
	}

	update := lu.DeepCopy()
	update.Status.Failed = failed

	err := r.client.Status().Update(ctx, update)

	return ctrl.Result{}, err
}

// Create the monitor that tracks a live update. If the live update
// spec changes, wipe out all accumulated state.
func (r *Reconciler) ensureMonitorExists(name string, obj *v1alpha1.LiveUpdate) *monitor {
	spec := obj.Spec
	m, ok := r.monitors[name]
	if ok && apicmp.DeepEqual(obj.Spec, m.spec) {
		return m
	}

	m = &monitor{
		manifestName: obj.Annotations[v1alpha1.AnnotationManifest],
		spec:         spec,
		sources:      make(map[string]*monitorSource),
		containers:   make(map[monitorContainerKey]monitorContainerStatus),
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
func (r *Reconciler) reconcileSources(ctx context.Context, monitor *monitor) (bool, error) {
	if len(monitor.spec.Sources) == 0 {
		return false, nil
	}

	hasChange := false
	for _, s := range monitor.spec.Sources {
		oneChange, err := r.reconcileOneSource(ctx, monitor, s)
		if err != nil {
			return false, err
		}
		if oneChange {
			hasChange = true
		}
	}
	return hasChange, nil
}

// Consume one Source object.
func (r *Reconciler) reconcileOneSource(ctx context.Context, monitor *monitor, source v1alpha1.LiveUpdateSource) (bool, error) {
	fwn := source.FileWatch
	imn := source.ImageMap

	var fw v1alpha1.FileWatch
	if fwn != "" {
		err := r.client.Get(ctx, types.NamespacedName{Name: fwn}, &fw)
		if err != nil {
			return false, err
		}
	}

	var im v1alpha1.ImageMap
	if imn != "" {
		err := r.client.Get(ctx, types.NamespacedName{Name: imn}, &im)
		if err != nil {
			return false, err
		}
	}

	events := fw.Status.FileEvents
	if len(events) == 0 || fwn == "" {
		return false, nil
	}

	mSource, ok := monitor.sources[fwn]
	if !ok {
		mSource = &monitorSource{
			modTimeByPath: make(map[string]metav1.MicroTime),
		}
		monitor.sources[fwn] = mSource
	}

	newImageStatus := im.Status
	imageChanged := false
	if imn != "" {
		imageChanged = mSource.lastImageStatus == nil ||
			!apicmp.DeepEqual(&newImageStatus, mSource.lastImageStatus)
		mSource.lastImageStatus = &im.Status
	}

	newLastFileEvent := events[len(events)-1]
	event := mSource.lastFileEvent
	fileWatchChanged := event == nil || !apicmp.DeepEqual(&newLastFileEvent, event)
	mSource.lastFileEvent = &newLastFileEvent

	if fileWatchChanged {
		// Consume all the file events.
		for _, event := range events {
			eventTime := event.Time.Time
			if newImageStatus.BuildStartTime != nil && newImageStatus.BuildStartTime.After(eventTime) {
				continue
			}

			for _, f := range event.SeenFiles {
				existing, ok := mSource.modTimeByPath[f]
				if !ok || existing.Time.Before(event.Time.Time) {
					mSource.modTimeByPath[f] = event.Time
				}
			}
		}
	}

	return fileWatchChanged || imageChanged, nil
}

// Consume the TriggerQueue.
// This isn't formally represented in the API right now, it's just
// a ConfigMap to pull attributes off of.
// Returns true if we saw any changes.
func (r *Reconciler) reconcileTriggerQueue(ctx context.Context, monitor *monitor) (bool, error) {
	queue, err := configmap.TriggerQueue(ctx, r.client)
	if err != nil {
		return false, client.IgnoreNotFound(err)
	}

	if monitor.lastTriggerQueue != nil && apicmp.DeepEqual(queue.Data, monitor.lastTriggerQueue.Data) {
		return false, nil
	}

	monitor.lastTriggerQueue = queue
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

	if ka == nil {
		monitor.lastKubernetesApplyStatus = nil
	} else {
		monitor.lastKubernetesApplyStatus = &(ka.Status)
	}

	monitor.lastKubernetesDiscovery = kd

	return changed, nil
}

// Go through all the file changes, and delete files that aren't relevant
// to the current build.
//
// Determining the current build is a bit tricky, but our
// order of preference is:
// 1) If we have an ImageMap.BuildStartedAt, this is the gold standard.
// 2) If there's no ImageMap, we prefer the KubernetesApply.LastApplyStartTime.
// 3) If there's no KubernetesApply, we prefer the oldest pod
//    in the filtered pod list.
func (r *Reconciler) garbageCollectFileChanges(res *k8sconv.KubernetesResource, monitor *monitor) {
	for _, source := range monitor.spec.Sources {
		fwn := source.FileWatch
		mSource, ok := monitor.sources[fwn]
		if !ok {
			continue
		}

		lastImageStatus := mSource.lastImageStatus
		var gcTime time.Time
		if lastImageStatus != nil && lastImageStatus.BuildStartTime != nil {
			gcTime = lastImageStatus.BuildStartTime.Time
		} else if res.ApplyStatus != nil {
			gcTime = res.ApplyStatus.LastApplyStartTime.Time
		} else {
			for _, pod := range res.FilteredPods {
				if gcTime.IsZero() || (!pod.CreatedAt.IsZero() && pod.CreatedAt.Time.Before(gcTime)) {
					gcTime = pod.CreatedAt.Time
				}
			}
		}

		if !gcTime.IsZero() {
			// Delete all file events that happened before the
			// latest build started.
			for p, t := range mSource.modTimeByPath {
				if gcTime.After(t.Time) {
					delete(mSource.modTimeByPath, p)
				}
			}

			// Delete all failures that happened before the
			// latest build started.
			//
			// This mechanism isn't perfect - for example, it will start resyncing
			// again to a container that's going to be replaced by the current
			// build. But we also can't determine if a container is going to be
			// replaced or not (particularly if the image didn't change).
			for key, c := range monitor.containers {
				if !c.failedLowWaterMark.IsZero() && gcTime.After(c.failedLowWaterMark.Time) {
					c.failedLowWaterMark = metav1.MicroTime{}
					c.failedReason = ""
					c.failedMessage = ""
					monitor.containers[key] = c
				}
			}
		}
	}
}

// Go through all the container monitors, and delete any that are no longer
// being selected. We don't care why they're not being selected.
func (r *Reconciler) garbageCollectMonitorContainers(res *k8sconv.KubernetesResource, monitor *monitor) {
	podsByKey := map[monitorContainerKey]bool{}
	for _, pod := range res.FilteredPods {
		podsByKey[monitorContainerKey{podName: pod.Name, namespace: pod.Namespace}] = true
	}

	for key := range monitor.containers {
		podKey := monitorContainerKey{podName: key.podName, namespace: key.namespace}
		if !podsByKey[podKey] {
			delete(monitor.containers, key)
		}
	}
}

// Visit all selected containers.
func (r *Reconciler) visitSelectedContainers(
	kSelector *v1alpha1.LiveUpdateKubernetesSelector,
	kResource *k8sconv.KubernetesResource,
	visit func(pod v1alpha1.Pod, c v1alpha1.Container) bool) {
	for _, pod := range kResource.FilteredPods {
		for _, c := range pod.Containers {
			if c.Name == "" {
				// ignore any blatantly invalid containers
				continue
			}

			// LiveUpdateKubernetesSelector must specify EITHER image OR container name
			if kSelector.Image != "" {
				imageRef, err := container.ParseNamed(c.Image)
				if err != nil || imageRef == nil || kSelector.Image != reference.FamiliarName(imageRef) {
					continue
				}
			} else if kSelector.ContainerName != c.Name {
				continue
			}
			stop := visit(pod, c)
			if stop {
				return
			}
		}
	}
}

func (r *Reconciler) dispatchStartBuildAction(ctx context.Context, lu *v1alpha1.LiveUpdate, filesChanged []string) {
	manifestName := lu.Annotations[v1alpha1.AnnotationManifest]
	spanID := lu.Annotations[v1alpha1.AnnotationSpanID]
	r.store.Dispatch(buildcontrols.BuildStartedAction{
		ManifestName:       model.ManifestName(manifestName),
		StartTime:          time.Now(),
		FilesChanged:       filesChanged,
		Reason:             model.BuildReasonFlagChangedFiles,
		SpanID:             logstore.SpanID(spanID),
		FullBuildTriggered: false,
	})

	buildcontrols.LogBuildEntry(ctx, buildcontrols.BuildEntry{
		Name:         model.ManifestName(manifestName),
		BuildReason:  model.BuildReasonFlagChangedFiles,
		FilesChanged: filesChanged,
	})
}

func (r *Reconciler) dispatchCompleteBuildAction(lu *v1alpha1.LiveUpdate, newStatus v1alpha1.LiveUpdateStatus) {
	manifestName := model.ManifestName(lu.Annotations[v1alpha1.AnnotationManifest])
	spanID := logstore.SpanID(lu.Annotations[v1alpha1.AnnotationSpanID])
	var err error
	if newStatus.Failed != nil {
		err = fmt.Errorf("%s", newStatus.Failed.Message)
	}
	imageTargetID := model.TargetID{
		Type: model.TargetTypeImage,
		Name: model.TargetName(apis.SanitizeName(lu.Spec.Selector.Kubernetes.Image)),
	}
	containerIDs := []container.ID{}
	for _, status := range newStatus.Containers {
		if status.Waiting == nil {
			containerIDs = append(containerIDs, container.ID(status.ContainerID))
		}
	}
	result := store.NewLiveUpdateBuildResult(imageTargetID, containerIDs)
	resultSet := store.BuildResultSet{imageTargetID: result}
	r.store.Dispatch(buildcontrols.NewBuildCompleteAction(manifestName, spanID, resultSet, err))
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

	manifestName := lu.Annotations[v1alpha1.AnnotationManifest]
	updateMode := lu.Annotations[liveupdate.AnnotationUpdateMode]
	inTriggerQueue := monitor.lastTriggerQueue != nil && manifestName != "" &&
		configmap.InTriggerQueue(monitor.lastTriggerQueue, types.NamespacedName{Name: manifestName})
	isUpdateModeManual := updateMode == liveupdate.UpdateModeManual
	isWaitingOnTrigger := false
	if isUpdateModeManual && !inTriggerQueue {
		// In manual mode, we should always wait for a trigger before live updating anything.
		isWaitingOnTrigger = true
	}

	r.garbageCollectFileChanges(kResource, monitor)
	r.garbageCollectMonitorContainers(kResource, monitor)

	// Go through all the container monitors, and check if any of them are unrecoverable.
	// If they are, it's not important to figure out why.
	r.visitSelectedContainers(kSelector, kResource, func(pod v1alpha1.Pod, c v1alpha1.Container) bool {
		cKey := monitorContainerKey{
			containerID: c.ID,
			podName:     pod.Name,
			namespace:   pod.Namespace,
		}

		cStatus, ok := monitor.containers[cKey]
		if ok && cStatus.failedReason != "" {
			status.Failed = createFailedState(lu, cStatus.failedReason, cStatus.failedMessage)
			return true
		}
		return false
	})

	if status.Failed != nil {
		return status
	}

	updateEventDispatched := false

	// Visit all containers, apply changes, and return their statuses.
	terminatedContainerPodName := ""
	hasAnyFilesToSync := false
	r.visitSelectedContainers(kSelector, kResource, func(pod v1alpha1.Pod, cInfo v1alpha1.Container) bool {
		c := liveupdates.Container{
			ContainerID:   container.ID(cInfo.ID),
			ContainerName: container.Name(cInfo.Name),
			PodID:         k8s.PodID(pod.Name),
			Namespace:     k8s.Namespace(pod.Namespace),
		}
		cKey := monitorContainerKey{
			containerID: cInfo.ID,
			podName:     pod.Name,
			namespace:   pod.Namespace,
		}

		highWaterMark := r.startedTime
		cStatus, ok := monitor.containers[cKey]
		if ok && !cStatus.lastFileTimeSynced.IsZero() {
			highWaterMark = cStatus.lastFileTimeSynced
		}

		// Determine the changed files.
		filesChanged := []string{}
		newHighWaterMark := highWaterMark
		newLowWaterMark := metav1.MicroTime{}
		for _, source := range monitor.sources {
			for f, t := range source.modTimeByPath {
				if t.After(highWaterMark.Time) {
					filesChanged = append(filesChanged, f)

					if newLowWaterMark.IsZero() || t.Before(&newLowWaterMark) {
						newLowWaterMark = t
					}

					if t.After(newHighWaterMark.Time) {
						newHighWaterMark = t
					}
				}
			}
		}

		// Sort the files so that they're deterministic.
		filesChanged = sliceutils.DedupedAndSorted(filesChanged)
		if len(filesChanged) > 0 {
			hasAnyFilesToSync = true
		}

		// Ignore completed pods/containers.
		// This is a bit tricky to handle correctly, but is handled at
		// the end of this function.
		if pod.Phase == string(v1.PodSucceeded) || pod.Phase == string(v1.PodFailed) || cInfo.State.Terminated != nil {
			if terminatedContainerPodName == "" {
				terminatedContainerPodName = pod.Name
			}
			return false
		}

		var waiting *v1alpha1.LiveUpdateContainerStateWaiting

		// We interpret "no container id" as a waiting state
		// (terminated states should have been caught above).
		if cInfo.State.Running == nil || cInfo.ID == "" {
			waiting = &v1alpha1.LiveUpdateContainerStateWaiting{
				Reason:  "ContainerWaiting",
				Message: "Waiting for container to start",
			}
		} else if isWaitingOnTrigger {
			waiting = &v1alpha1.LiveUpdateContainerStateWaiting{
				Reason:  "Trigger",
				Message: "Only updates on manual trigger",
			}
		}

		// Create a plan to update the container.
		filesApplied := false
		var oneUpdateStatus v1alpha1.LiveUpdateStatus
		plan, failed := r.createLiveUpdatePlan(lu.Spec, filesChanged)
		if failed != nil {
			// The plan told us to stop updating - this container is unrecoverable.
			oneUpdateStatus.Failed = failed
		} else if len(plan.SyncPaths) == 0 {
			// The plan told us that there are no updates to do.
			oneUpdateStatus.Containers = []v1alpha1.LiveUpdateContainerStatus{{
				ContainerName:      cInfo.Name,
				ContainerID:        cInfo.ID,
				PodName:            pod.Name,
				Namespace:          pod.Namespace,
				LastFileTimeSynced: cStatus.lastFileTimeSynced,
				Waiting:            waiting,
			}}
		} else if cInfo.State.Waiting != nil && cInfo.State.Waiting.Reason == "CrashLoopBackOff" {
			// At this point, the plan told us that we have some files to sync.
			// Check if the container is in a state to receive those updates.

			// If the container is crashlooping, that means it might not be up long enough
			// to be able to receive a live-update. Treat this as an unrecoverable failure case.
			oneUpdateStatus.Failed = createFailedState(lu, "CrashLoopBackOff",
				fmt.Sprintf("Cannot live update because container crashing. Pod: %s", pod.Name))

		} else if waiting != nil {
			// Mark the container as waiting, so we have a record of it. No need to sync any files.
			oneUpdateStatus.Containers = []v1alpha1.LiveUpdateContainerStatus{{
				ContainerName:      cInfo.Name,
				ContainerID:        cInfo.ID,
				PodName:            pod.Name,
				Namespace:          pod.Namespace,
				LastFileTimeSynced: cStatus.lastFileTimeSynced,
				Waiting:            waiting,
			}}
		} else {
			// Log progress and treat this as an update in the engine state.
			if !updateEventDispatched {
				updateEventDispatched = true
				r.dispatchStartBuildAction(ctx, lu, filesChanged)
			}

			// Apply the change to the container.
			oneUpdateStatus = r.applyInternal(ctx, lu.Spec, Input{
				IsDC:               false, // update this once we support DockerCompose in the API.
				ChangedFiles:       plan.SyncPaths,
				Containers:         []liveupdates.Container{c},
				LastFileTimeSynced: newHighWaterMark,
			})
			filesApplied = true
		}

		// Merge the status from the single update into the overall liveupdate status.
		adjustFailedStateTimestamps(lu, &oneUpdateStatus)

		// Update the monitor based on the result of the applied changes.
		if oneUpdateStatus.Failed != nil {
			cStatus.failedReason = oneUpdateStatus.Failed.Reason
			cStatus.failedMessage = oneUpdateStatus.Failed.Message
			cStatus.failedLowWaterMark = newLowWaterMark
		} else if filesApplied {
			cStatus.lastFileTimeSynced = newHighWaterMark
		}
		monitor.containers[cKey] = cStatus

		// Update the status based on the result of the applied changes.
		if oneUpdateStatus.Failed != nil {
			status.Failed = oneUpdateStatus.Failed
			status.Containers = nil
			return true
		}

		status.Containers = append(status.Containers, oneUpdateStatus.Containers...)
		return false
	})

	// If the only containers we're connected to are terminated containers,
	// there are two cases we need to worry about:
	//
	// 1) The pod has completed, and will never run again (like a Job).
	// 2) This is an old pod, and we're waiting for the new pod to rollout.
	//
	// We don't really have a great way to distinguish between these two cases.
	//
	// If we get to the end of this loop and haven't found any "live" pods,
	// we assume we're in state (1) (to prevent waiting forever).
	if status.Failed == nil && terminatedContainerPodName != "" &&
		hasAnyFilesToSync && len(status.Containers) == 0 {
		status.Failed = createFailedState(lu, "Terminated",
			fmt.Sprintf("Container for live update is stopped. Pod name: %s", terminatedContainerPodName))
	}

	if updateEventDispatched {
		r.dispatchCompleteBuildAction(lu, status)
	}

	return status
}

func (r *Reconciler) createLiveUpdatePlan(spec v1alpha1.LiveUpdateSpec, filesChanged []string) (liveupdates.LiveUpdatePlan, *v1alpha1.LiveUpdateStateFailed) {
	plan, err := liveupdates.NewLiveUpdatePlan(spec, filesChanged)
	if err != nil {
		return plan, &v1alpha1.LiveUpdateStateFailed{
			Reason:  "UpdateStopped",
			Message: fmt.Sprintf("No update plan: %v", err),
		}
	}

	if len(plan.NoMatchPaths) > 0 {
		return plan, &v1alpha1.LiveUpdateStateFailed{
			Reason: "UpdateStopped",
			Message: fmt.Sprintf("Found file(s) not matching any sync (files: %s)",
				ospath.FormatFileChangeList(plan.NoMatchPaths)),
		}
	}

	// If any changed files match a FallBackOn file, fall back to next BuildAndDeployer
	if len(plan.StopPaths) != 0 {
		return plan, &v1alpha1.LiveUpdateStateFailed{
			Reason:  "UpdateStopped",
			Message: fmt.Sprintf("Detected change to stop file %q", plan.StopPaths[0]),
		}
	}
	return plan, nil
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
	names := liveupdates.ContainerDisplayNames(containers)
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
		l.Infof("Will delete %d file(s) from container%s: %s", len(toRemove), suffix, names)
		for _, pm := range toRemove {
			l.Infof("- '%s' (matched local path: '%s')", pm.ContainerPath, pm.LocalPath)
		}
	}

	if len(toArchive) > 0 {
		l.Infof("Will copy %d file(s) to container%s: %s", len(toArchive), suffix, names)
		for _, pm := range toArchive {
			l.Infof("- %s", pm.PrettyStr())
		}
	}

	g, ctx := errgroup.WithContext(ctx)
	result.Containers = make([]v1alpha1.LiveUpdateContainerStatus, len(containers))
	errors := make([]*v1alpha1.LiveUpdateStateFailed, len(containers))
	for i, cInfo := range containers {
		i, cInfo := i, cInfo
		var err error
		g.Go(func() error {
			// TODO(nick): We should try to distinguish between cases where the tar writer
			// fails (which is recoverable) vs when the server-side unpacking
			// fails (which may not be recoverable).
			archive := build.TarArchiveForPaths(ctx, toArchive, nil)
			err = cu.UpdateContainer(ctx, cInfo, archive,
				build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)
			_ = archive.Close()

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
					// Container failed to update because of an error within user configuration.
					// Keep running updates -- we want all containers to have the same files on them
					// even if the Runs don't succeed
					logger.Get(ctx).Infof("  → Failed to update container %s: run step %q failed with exit code: %d",
						cInfo.DisplayName(), runFail.Cmd.String(), runFail.ExitCode)
					cStatus.LastExecError = err.Error()
					errors[i] = &v1alpha1.LiveUpdateStateFailed{
						Reason:  RunStepFailure,
						Message: fmt.Sprintf("Updating pod %s: %v", cStatus.PodName, err),
					}
				} else {
					// Something went wrong with this update, and it's NOT the user's fault--
					// likely an infrastructure error. Trigger a fallback to full build.
					errors[i] = &v1alpha1.LiveUpdateStateFailed{
						Reason:  "UpdateFailed",
						Message: fmt.Sprintf("Updating pod %s: %v", cStatus.PodName, err),
					}
				}
			} else {
				logger.Get(ctx).Infof("  → Container %s updated!", cInfo.DisplayName())
			}
			result.Containers[-i] = cStatus
			return nil
		})
	}

	_ = g.Wait()

	var runError int
	for _, e := range errors {
		if e == nil {
			continue
		}
		if e.Reason == RunStepFailure {
			runError += 1
			continue
		}
		result.Failed = e
	}

	if runError > 0 && runError != len(result.Containers) {
		// Some builds succeeded, but at least one failed at the run step (likely a user error).
		// We may have inconsistent state--bail, and fall back to full build.
		result.Failed = &v1alpha1.LiveUpdateStateFailed{
			Reason: "PodsInconsistent",
			Message: fmt.Sprintf("Pods in inconsistent state. Success: %v. Failures: '%v'; ",
				result.Containers, errors),
		}
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
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.enqueueTriggerQueue))

	return b, nil
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

	// We can only trigger liveupdates that have run once, so search
	// through the map of known liveupdates
	names := configmap.NamesInTriggerQueue(cm)
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	requests := []reconcile.Request{}
	for name, monitor := range r.monitors {
		if nameSet[monitor.manifestName] {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		}
	}
	return requests
}

// indexLiveUpdate returns keys of objects referenced _by_ the LiveUpdate object for reverse lookup including:
//  - FileWatch
//  - ImageMapName
// 	- KubernetesDiscovery
//	- KubernetesApply
func indexLiveUpdate(obj ctrlclient.Object) []indexer.Key {
	lu := obj.(*v1alpha1.LiveUpdate)
	var result []indexer.Key

	for _, s := range lu.Spec.Sources {
		fwn := s.FileWatch
		imn := s.ImageMap
		if fwn != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      fwn,
				},
				GVK: fwGVK,
			})
		}

		if imn != "" {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{
					Namespace: lu.Namespace,
					Name:      imn,
				},
				GVK: imageMapGVK,
			})
		}
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
	}
	return result
}
