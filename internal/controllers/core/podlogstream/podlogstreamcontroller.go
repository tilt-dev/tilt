package podlogstream

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/jonboulle/clockwork"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/logger"
)

var podLogHealthCheck = 15 * time.Second
var podLogReconnectGap = 2 * time.Second

const maxDebounceDuration = 5 * time.Minute

var clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

// Reconciles the PodLogStream API object.
//
// Collects logs from deployed containers.
type Controller struct {
	ctx       context.Context
	client    ctrlclient.Client
	indexer   *indexer.Indexer
	st        store.RStore
	kClient   k8s.Client
	podSource *PodSource
	mu        sync.Mutex
	clock     clockwork.Clock

	watches         map[podLogKey]*podLogWatch
	hasClosedStream map[podLogKey]bool
	statuses        map[types.NamespacedName]*PodLogStreamStatus
}

var _ reconcile.Reconciler = &Controller{}
var _ store.TearDowner = &Controller{}

func NewController(ctx context.Context, client ctrlclient.Client, scheme *runtime.Scheme, st store.RStore, kClient k8s.Client, podSource *PodSource, clock clockwork.Clock) *Controller {
	return &Controller{
		ctx:             ctx,
		client:          client,
		indexer:         indexer.NewIndexer(scheme, indexPodLogStreamForTiltAPI),
		st:              st,
		kClient:         kClient,
		podSource:       podSource,
		watches:         make(map[podLogKey]*podLogWatch),
		hasClosedStream: make(map[podLogKey]bool),
		statuses:        make(map[types.NamespacedName]*PodLogStreamStatus),
		clock:           clock,
	}
}

// Filter containers based on the inclusions/exclusions in the PodLogStream spec.
func (c *Controller) filterContainers(stream *PodLogStream, containers []v1alpha1.Container) []v1alpha1.Container {
	if len(stream.Spec.OnlyContainers) > 0 {
		only := make(map[container.Name]bool, len(stream.Spec.OnlyContainers))
		for _, name := range stream.Spec.OnlyContainers {
			only[container.Name(name)] = true
		}

		result := []v1alpha1.Container{}
		for _, c := range containers {
			if only[container.Name(c.Name)] {
				result = append(result, c)
			}
		}
		return result
	}

	if len(stream.Spec.IgnoreContainers) > 0 {
		ignore := make(map[container.Name]bool, len(stream.Spec.IgnoreContainers))
		for _, name := range stream.Spec.IgnoreContainers {
			ignore[container.Name(name)] = true
		}

		result := []v1alpha1.Container{}
		for _, c := range containers {
			if !ignore[container.Name(c.Name)] {
				result = append(result, c)
			}
		}
		return result
	}
	return containers
}

func (c *Controller) TearDown(ctx context.Context) {
	c.podSource.TearDown()
}

func (c *Controller) shouldStreamContainerLogs(pod *v1.Pod, co v1alpha1.Container, key podLogKey) bool {
	if co.ID == "" {
		return false
	}

	isTerminating := co.State.Terminated != nil ||
		(pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero())
	if isTerminating && c.hasClosedStream[key] {
		return false
	}

	if co.State.Running == nil && co.State.Terminated == nil {
		// nothing to stream for containers in waiting state
		return false
	}

	return true

}

// Reconcile the given stream against what we're currently tracking.
func (c *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stream := &PodLogStream{}
	streamName := req.NamespacedName
	err := c.client.Get(ctx, req.NamespacedName, stream)
	c.indexer.OnReconcile(req.NamespacedName, stream)
	if apierrors.IsNotFound(err) {
		// handleReconcileRequest returns errors that should be published
		// to status.error. But the pod log stream is deleted! so can ignore.
		_ = c.podSource.handleReconcileRequest(ctx, req.NamespacedName, stream)
		c.deleteStreams(streamName)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	result := reconcile.Result{}
	ctx = store.MustObjectLogHandler(ctx, c.st, stream)
	err = c.podSource.handleReconcileRequest(ctx, streamName, stream)
	if err != nil {
		c.setErrorStatus(streamName, err)
	} else {
		podNN := types.NamespacedName{Name: stream.Spec.Pod, Namespace: stream.Spec.Namespace}
		pod, err := c.kClient.PodFromInformerCache(ctx, podNN)
		deleting := false
		if err != nil && apierrors.IsNotFound(err) {
			c.deleteStreams(streamName)
			deleting = true
		}

		if pod == nil || apierrors.IsNotFound(err) {
			c.setErrorStatus(streamName, fmt.Errorf("pod not found: %s", podNN))
		} else if err != nil {
			c.setErrorStatus(streamName, fmt.Errorf("reading pod: %v", err))
		} else if !deleting {
			result = c.addOrUpdateContainerWatches(ctx, streamName, stream, podNN, pod)
		}
	}

	err = c.maybeUpdateObjectStatus(ctx, streamName, stream)
	if err != nil {
		return reconcile.Result{}, err
	}

	return result, nil
}

func (c *Controller) addOrUpdateContainerWatches(ctx context.Context, streamName types.NamespacedName, stream *v1alpha1.PodLogStream, podNN types.NamespacedName, pod *v1.Pod) reconcile.Result {
	initContainers := c.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.InitContainerStatuses))
	runContainers := c.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.ContainerStatuses))
	containers := []v1alpha1.Container{}
	containers = append(containers, initContainers...)
	containers = append(containers, runContainers...)
	c.ensureStatusActive(streamName, containers)

	result := reconcile.Result{}
	containerWatches := make(map[podLogKey]bool)
	for i, co := range containers {
		// Key the log watcher by the container id, so we auto-restart the
		// watching if the container crashes.
		key := podLogKey{
			streamName: streamName,
			podID:      k8s.PodID(podNN.Name),
			cID:        container.ID(co.ID),
		}
		if !c.shouldStreamContainerLogs(pod, co, key) {
			continue
		}

		isInitContainer := i < len(initContainers)

		// We don't want to clutter the logs with a container name
		// if it's unambiguous what container we're looking at.
		//
		// Long-term, we should make the container name a log field
		// and have better ways to display it visually.
		shouldPrefix := isInitContainer || len(runContainers) > 1

		containerWatches[key] = true

		existing, isActive := c.watches[key]

		// Only stream logs that have happened since Tilt started.
		//
		// TODO(nick): We should really record when we started the `kubectl apply`,
		// and only stream logs since that happened.
		startWatchTime := time.Time{}
		if stream.Spec.SinceTime != nil {
			startWatchTime = stream.Spec.SinceTime.Time
		}
		debounce := time.Second

		if isActive {
			if existing.ctx.Err() == nil {
				// The active pod watcher is still tailing the logs,
				// nothing to do.
				continue
			}

			// The active pod watcher got canceled somehow,
			// so we need to create a new one that picks up
			// where it left off.
			<-existing.doneCh
			startWatchTime = existing.doneWatchTime
			debounce = existing.debounce
			c.hasClosedStream[key] = true
			if co.State.Terminated != nil {
				c.mutateContainerStatus(streamName, container.Name(co.Name), func(cs *ContainerLogStreamStatus) {
					cs.Terminated = true
					cs.Active = false
					cs.Error = ""
				})
				continue
			}

			if c.clock.Since(existing.doneWatchTime) < debounce {
				requeueAfter := debounce - c.clock.Since(existing.doneWatchTime)
				if requeueAfter > result.RequeueAfter {
					result.RequeueAfter = requeueAfter
				}
				continue
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		w := &podLogWatch{
			streamName:     streamName,
			ctx:            ctx,
			cancel:         cancel,
			podID:          k8s.PodID(podNN.Name),
			cName:          container.Name(co.Name),
			namespace:      k8s.Namespace(podNN.Namespace),
			startWatchTime: startWatchTime,
			debounce:       debounce,
			doneCh:         make(chan struct{}),
			shouldPrefix:   shouldPrefix,
		}
		c.watches[key] = w

		go c.consumeLogs(w, c.st)
	}

	for key, watch := range c.watches {
		_, inState := containerWatches[key]
		if !inState && key.streamName == streamName {
			watch.cancel()
			delete(c.watches, key)
		}
	}
	return result
}

// Delete all the streams generated by the named API object
func (c *Controller) deleteStreams(streamName types.NamespacedName) {
	for k, watch := range c.watches {
		if k.streamName != streamName {
			continue
		}
		watch.cancel()
		delete(c.watches, k)
	}

	delete(c.statuses, streamName)
}

// Consume logs in a goroutine.
//
// Note that this does NOT have the lock, so anything in this function
// needs to be careful about accessing shared state.
func (c *Controller) consumeLogs(watch *podLogWatch, st store.RStore) {
	pID := watch.podID
	ctx := watch.ctx
	containerName := watch.cName
	var exitError error

	defer func() {
		// When the log streaming ends, log it and report the status change to the
		// apiserver.
		c.mu.Lock()
		c.mutateContainerStatus(watch.streamName, containerName, func(cs *ContainerLogStreamStatus) {
			cs.Active = false
			if exitError == nil {
				cs.Error = ""
			} else {
				cs.Error = exitError.Error()
			}
		})
		c.mu.Unlock()
		c.podSource.requeueStream(watch.streamName)

		if exitError != nil {
			// TODO(nick): Should this be Warnf/Errorf?
			logger.Get(ctx).Infof("Error streaming %s logs: %v", pID, exitError)
			watch.debounce = watch.debounce * 2
			if watch.debounce > maxDebounceDuration {
				watch.debounce = maxDebounceDuration
			}
		} else {
			watch.debounce = time.Second
		}

		watch.doneWatchTime = c.clock.Now()
		close(watch.doneCh)
		watch.cancel()
	}()

	ns := watch.namespace
	startReadTime := watch.startWatchTime
	if watch.shouldPrefix {
		prefix := fmt.Sprintf("[%s] ", watch.cName)
		ctx = logger.WithLogger(ctx, logger.NewPrefixedLogger(prefix, logger.Get(ctx)))
	}

	retry := true
	for retry {
		retry = false
		ctx, cancel := context.WithCancel(ctx)
		readCloser, err := c.kClient.ContainerLogs(ctx, pID, containerName, ns, startReadTime)
		if err != nil {
			if ctx.Err() == nil {
				exitError = err
			}
			cancel()
			return
		}

		reader := runtimelog.NewHardCancelReader(ctx, readCloser)
		reader.Now = c.clock.Now

		// A hacky workaround for
		// https://github.com/tilt-dev/tilt/issues/3908
		// Every 15 seconds, check to see if the logs have stopped streaming.
		// If they have, reconnect to the log stream.
		done := make(chan bool)
		go func() {
			ticker := c.clock.NewTicker(podLogHealthCheck)
			for {
				select {
				case <-done:
					return

				case <-ticker.Chan():
					lastRead := reader.LastReadTime()
					if lastRead.IsZero() || c.clock.Since(lastRead) < podLogHealthCheck {
						continue
					}

					retry = true

					// Start reading 2 seconds after the last read.
					//
					// In the common case (where we just haven't gotten any logs in the
					// last 15 seconds), this will ensure we don't duplicate logs.
					//
					// In the uncommon case (where the Kuberentes log buffer exceeded 10MB
					// and got rotated), this will create a 2 second gap in the log, but
					// we think this is acceptable to avoid the duplicate case.
					startReadTime = lastRead.Add(podLogReconnectGap)
					cancel()
					return
				}
			}
		}()

		c.mu.Lock()
		c.mutateContainerStatus(watch.streamName, containerName, func(cs *ContainerLogStreamStatus) {
			cs.Active = true
			cs.Error = ""
		})
		c.mu.Unlock()
		c.podSource.requeueStream(watch.streamName)

		_, err = io.Copy(logger.Get(ctx).Writer(logger.InfoLvl), reader)
		_ = readCloser.Close()
		close(done)

		wasCanceledUpstream := ctx.Err() != nil
		cancel()

		if !retry && err != nil && !wasCanceledUpstream {
			exitError = err
			return
		}
	}
}

// Set up the status object for a particular stream, tracking each container individually.
func (c *Controller) ensureStatusActive(streamName types.NamespacedName, containers []v1alpha1.Container) {
	status, ok := c.statuses[streamName]
	if !ok {
		status = &PodLogStreamStatus{}
		c.statuses[streamName] = status
	}

	// Make sure the container names are right. If they're not, delete everything and recreate.
	isMatching := len(containers) == len(status.ContainerStatuses)
	if isMatching {
		for i, cs := range status.ContainerStatuses {
			if string(containers[i].Name) != cs.Name {
				isMatching = false
				break
			}
		}
	}

	if isMatching {
		return
	}

	statuses := make([]ContainerLogStreamStatus, 0, len(containers))
	for _, c := range containers {
		statuses = append(statuses, ContainerLogStreamStatus{
			Name: string(c.Name),
		})
	}
	status.ContainerStatuses = statuses
	status.Error = ""
}

// Modify the status of a container log stream.
func (c *Controller) mutateContainerStatus(streamName types.NamespacedName, containerName container.Name, mutate func(*ContainerLogStreamStatus)) {
	status, ok := c.statuses[streamName]
	if !ok {
		return
	}

	for i, cs := range status.ContainerStatuses {
		if cs.Name != string(containerName) {
			continue
		}

		mutate(&cs)
		status.ContainerStatuses[i] = cs
	}
}

// Set the pod log stream to an error status
func (c *Controller) setErrorStatus(streamName types.NamespacedName, err error) {
	status, ok := c.statuses[streamName]
	if !ok {
		status = &PodLogStreamStatus{}
		c.statuses[streamName] = status
	}
	if err == nil {
		status.Error = ""
	} else {
		status.Error = err.Error()
	}
}

// Update the server with the current container status.
func (c *Controller) maybeUpdateObjectStatus(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.PodLogStream) error {
	status, ok := c.statuses[nn]
	if !ok || apicmp.DeepEqual(*status, obj.Status) {
		return nil
	}

	oldError := obj.Status.Error
	newError := status.Error
	update := obj.DeepCopy()
	status.DeepCopyInto(&update.Status)

	err := c.client.Status().Update(c.ctx, update)
	if err != nil {
		return err
	}

	// Only show new errors.
	//
	// Don't show errors about the pod not being found. Those are pretty common
	// when the pod hasn't shown up in the informer cache yet.
	if newError != "" && !strings.HasPrefix(newError, "pod not found:") && newError != oldError {
		logger.Get(ctx).Errorf("podlogstream %s: %s", obj.Name, newError)
	}
	return nil
}

func (c *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&PodLogStream{}).
		Watches(c.podSource, handler.Funcs{})

	return b, nil
}

type podLogWatch struct {
	ctx    context.Context
	cancel func()

	streamName     types.NamespacedName
	podID          k8s.PodID
	namespace      k8s.Namespace
	cName          container.Name
	startWatchTime time.Time
	doneWatchTime  time.Time
	debounce       time.Duration
	doneCh         chan struct{}

	shouldPrefix bool // if true, we'll prefix logs with the container name
}

type podLogKey struct {
	streamName types.NamespacedName
	podID      k8s.PodID
	cID        container.ID
}

// indexPodLogStreamForTiltAPI indexes a PodLogStream object and returns keys
// for objects from the Tilt apiserver that it watches.
//
// See also: indexPodLogStreamForKubernetes which indexes a PodLogStream object
// and returns keys for objects from the Kubernetes cluster that it watches via
// PodSource.
func indexPodLogStreamForTiltAPI(obj ctrlclient.Object) []indexer.Key {
	var results []indexer.Key
	pls := obj.(*v1alpha1.PodLogStream)
	if pls != nil && pls.Spec.Cluster != "" {
		results = append(results, indexer.Key{
			Name: types.NamespacedName{Namespace: pls.Namespace, Name: pls.Spec.Cluster},
			GVK:  clusterGVK,
		})
	}
	return results
}
