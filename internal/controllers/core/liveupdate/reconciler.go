package liveupdate

import (
	"context"
	"fmt"

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
	"github.com/tilt-dev/tilt/internal/store"
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
	}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	lu := &v1alpha1.LiveUpdate{}
	err := r.client.Get(ctx, req.NamespacedName, lu)
	r.indexer.OnReconcile(req.NamespacedName, lu)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("liveupdate reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || lu.ObjectMeta.DeletionTimestamp != nil {
		r.store.Dispatch(liveupdates.NewLiveUpdateDeleteAction(req.Name))
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.store.Dispatch(liveupdates.NewLiveUpdateUpsertAction(lu))

	return ctrl.Result{}, nil
}

// TODO(nick): Merge this with LiveUpdateStatus,
// which will provide fuller status reporting.
type Status struct {
	// We failed to copy files to the container, but
	// we don't know why.
	UnknownError error

	// The exec command in the container failed.
	// This can often mean a compiler error that the user
	// can fix with more live-updates, so don't consider this
	// a "permanent" failure.
	ExecError error
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

	status := r.forceApplyInternal(ctx, nn, spec, input)

	var obj v1alpha1.LiveUpdate
	err := r.client.Get(ctx, nn, &obj)
	if err != nil {
		return v1alpha1.LiveUpdateStatus{}, err
	}

	// Check to see if this is a state transition.
	if status.Failed != nil {
		transitionTime := apis.NowMicro()
		if obj.Status.Failed != nil && obj.Status.Failed.Reason == status.Failed.Reason {
			// If the reason hasn't changed, don't treat this as a transition.
			transitionTime = obj.Status.Failed.LastTransitionTime
		}
		status.Failed.LastTransitionTime = transitionTime
	}

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

// Like ForceApply, but doesn't write the status to the apiserver.
func (r *Reconciler) forceApplyInternal(
	ctx context.Context,
	nn types.NamespacedName,
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

	filter := input.Filter
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
		archive := build.TarArchiveForPaths(ctx, toArchive, filter)
		err = cu.UpdateContainer(ctx, cInfo, archive,
			build.PathMappingsToContainerPaths(toRemove), boiledSteps, hotReload)

		cStatus := v1alpha1.LiveUpdateContainerStatus{
			ContainerName: cInfo.ContainerName.String(),
			ContainerID:   cInfo.ContainerID.String(),
			PodName:       cInfo.PodID.String(),
			Namespace:     cInfo.Namespace.String(),

			// TODO(nick): Pass in LastFileTimeSynced from the FileWatch events.
			LastFileTimeSynced: apis.NowMicro(),
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

	if lu.Spec.FileWatchName != "" {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{
				Namespace: lu.Namespace,
				Name:      lu.Spec.FileWatchName,
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
