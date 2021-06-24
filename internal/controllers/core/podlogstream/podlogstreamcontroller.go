package podlogstream

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/engine/runtimelog"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/logger"
)

var podLogHealthCheck = 15 * time.Second
var podLogReconnectGap = 2 * time.Second

// Reconciles the PodLogStream API object.
//
// Collects logs from deployed containers.
type Controller struct {
	ctx       context.Context
	client    ctrlclient.Client
	st        store.RStore
	kClient   k8s.Client
	podSource *PodSource
	mu        sync.Mutex

	watches         map[podLogKey]PodLogWatch
	hasClosedStream map[podLogKey]bool
	statuses        map[types.NamespacedName]*PodLogStreamStatus
	lastUpdate      map[types.NamespacedName]*PodLogStreamStatus

	newTicker func(d time.Duration) *time.Ticker
	since     func(t time.Time) time.Duration
	now       func() time.Time
}

var _ reconcile.Reconciler = &Controller{}
var _ store.TearDowner = &Controller{}

func NewController(ctx context.Context, client ctrlclient.Client, st store.RStore, kClient k8s.Client, podSource *PodSource) *Controller {
	return &Controller{
		ctx:             ctx,
		client:          client,
		st:              st,
		kClient:         kClient,
		podSource:       podSource,
		watches:         make(map[podLogKey]PodLogWatch),
		hasClosedStream: make(map[podLogKey]bool),
		statuses:        make(map[types.NamespacedName]*PodLogStreamStatus),
		lastUpdate:      make(map[types.NamespacedName]*PodLogStreamStatus),
		newTicker:       time.NewTicker,
		since:           time.Since,
		now:             time.Now,
	}
}

// Filter containers based on the inclusions/exclusions in the PodLogStream spec.
func (m *Controller) filterContainers(stream *PodLogStream, containers []v1alpha1.Container) []v1alpha1.Container {
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

func (m *Controller) shouldStreamContainerLogs(c v1alpha1.Container, key podLogKey) bool {
	if c.ID == "" {
		return false
	}

	if c.State.Terminated != nil && m.hasClosedStream[key] {
		return false
	}

	if c.State.Running == nil && c.State.Terminated == nil {
		// nothing to stream for containers in waiting state
		return false
	}

	return true

}

// Reconcile the given stream against what we're currently tracking.
func (r *Controller) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	stream := &PodLogStream{}
	streamName := req.NamespacedName
	err := r.client.Get(ctx, req.NamespacedName, stream)
	if apierrors.IsNotFound(err) {
		r.podSource.handleReconcileRequest(ctx, req.NamespacedName, stream)
		r.deleteStreams(streamName)
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	ctx = store.MustObjectLogHandler(ctx, r.st, stream)
	r.podSource.handleReconcileRequest(ctx, req.NamespacedName, stream)

	podNN := types.NamespacedName{Name: stream.Spec.Pod, Namespace: stream.Spec.Namespace}
	pod, err := r.kClient.PodFromInformerCache(ctx, podNN)
	if (err != nil && apierrors.IsNotFound(err)) ||
		(pod != nil && pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero()) {
		r.deleteStreams(streamName)
		return reconcile.Result{}, nil
	} else if err != nil {
		logger.Get(ctx).Debugf("streaming logs: %v", err)
		return reconcile.Result{}, err
	} else if pod == nil {
		logger.Get(ctx).Debugf("streaming logs: pod not found: %s", podNN)
		return reconcile.Result{}, nil
	}

	initContainers := r.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.InitContainerStatuses))
	runContainers := r.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.ContainerStatuses))
	containers := []v1alpha1.Container{}
	containers = append(containers, initContainers...)
	containers = append(containers, runContainers...)
	r.ensureStatus(streamName, containers)

	containerWatches := make(map[podLogKey]bool)
	for i, c := range containers {
		// Key the log watcher by the container id, so we auto-restart the
		// watching if the container crashes.
		key := podLogKey{
			streamName: streamName,
			podID:      k8s.PodID(podNN.Name),
			cID:        container.ID(c.ID),
		}
		if !r.shouldStreamContainerLogs(c, key) {
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

		existing, isActive := r.watches[key]

		// Only stream logs that have happened since Tilt started.
		//
		// TODO(nick): We should really record when we started the `kubectl apply`,
		// and only stream logs since that happened.
		startWatchTime := time.Time{}
		if stream.Spec.SinceTime != nil {
			startWatchTime = stream.Spec.SinceTime.Time
		}

		if isActive {
			if existing.ctx.Err() == nil {
				// The active pod watcher is still tailing the logs,
				// nothing to do.
				continue
			}

			// The active pod watcher got canceled somehow,
			// so we need to create a new one that picks up
			// where it left off.
			startWatchTime = <-existing.terminationTime
			r.hasClosedStream[key] = true
			if c.State.Terminated != nil {
				r.mutateStatus(streamName, container.Name(c.Name), func(cs *ContainerLogStreamStatus) {
					cs.Terminated = true
					cs.Active = false
					cs.Error = ""
				})
				continue
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		w := PodLogWatch{
			streamName:      streamName,
			ctx:             ctx,
			cancel:          cancel,
			podID:           k8s.PodID(podNN.Name),
			cName:           container.Name(c.Name),
			namespace:       k8s.Namespace(podNN.Namespace),
			startWatchTime:  startWatchTime,
			terminationTime: make(chan time.Time, 1),
			shouldPrefix:    shouldPrefix,
		}
		r.watches[key] = w

		go r.consumeLogs(w, r.st)
	}

	for key, watch := range r.watches {
		_, inState := containerWatches[key]
		if !inState && key.streamName == streamName {
			watch.cancel()
			delete(r.watches, key)
		}
	}

	r.updateStatus(streamName)

	return reconcile.Result{}, nil
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

	c.mu.Lock()
	delete(c.statuses, streamName)
	c.mu.Unlock()
}

func (m *Controller) consumeLogs(watch PodLogWatch, st store.RStore) {
	pID := watch.podID
	ctx := watch.ctx
	containerName := watch.cName
	var exitError error

	defer func() {
		// When the log streaming ends, log it and report the status change to the
		// apiserver.
		m.mutateStatus(watch.streamName, containerName, func(cs *ContainerLogStreamStatus) {
			cs.Active = false
			if exitError == nil {
				cs.Error = ""
			} else {
				cs.Error = exitError.Error()
			}
		})
		m.updateStatus(watch.streamName)

		if exitError != nil {
			// TODO(nick): Should this be Warnf/Errorf?
			logger.Get(ctx).Infof("Error streaming %s logs: %v", pID, exitError)
		}

		watch.terminationTime <- m.now()
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
		readCloser, err := m.kClient.ContainerLogs(ctx, pID, containerName, ns, startReadTime)
		if err != nil {
			cancel()
			exitError = err
			return
		}

		reader := runtimelog.NewHardCancelReader(ctx, readCloser)
		reader.Now = m.now

		// A hacky workaround for
		// https://github.com/tilt-dev/tilt/issues/3908
		// Every 15 seconds, check to see if the logs have stopped streaming.
		// If they have, reconnect to the log stream.
		done := make(chan bool)
		go func() {
			ticker := m.newTicker(podLogHealthCheck)
			for {
				select {
				case <-done:
					return

				case <-ticker.C:
					lastRead := reader.LastReadTime()
					if lastRead.IsZero() || m.since(lastRead) < podLogHealthCheck {
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

		m.mutateStatus(watch.streamName, containerName, func(cs *ContainerLogStreamStatus) {
			cs.Active = true
			cs.Error = ""
		})
		m.updateStatus(watch.streamName)

		_, err = io.Copy(logger.Get(ctx).Writer(logger.InfoLvl), reader)
		_ = readCloser.Close()
		close(done)
		cancel()

		if !retry && err != nil && ctx.Err() == nil {
			exitError = err
			return
		}
	}
}

// Set up the status object for a particular stream, tracking each container individually.
func (r *Controller) ensureStatus(streamName types.NamespacedName, containers []v1alpha1.Container) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[streamName]
	if !ok {
		status = &PodLogStreamStatus{}
		r.statuses[streamName] = status
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
}

// Modify the status of a container log stream.
func (r *Controller) mutateStatus(streamName types.NamespacedName, containerName container.Name, mutate func(*ContainerLogStreamStatus)) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[streamName]
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

// Update the server with the current container status.
func (r *Controller) updateStatus(streamName types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status, ok := r.statuses[streamName]
	if !ok {
		return
	}

	lastUpdate, hasLastUpdate := r.lastUpdate[streamName]
	if hasLastUpdate && equality.Semantic.DeepEqual(status, lastUpdate) {
		return
	}

	stream := &PodLogStream{}
	err := r.client.Get(r.ctx, streamName, stream)
	if err != nil {
		return
	}

	status.DeepCopyInto(&stream.Status)
	err = r.client.Status().Update(r.ctx, stream)
	if err != nil {
		return
	}
	r.lastUpdate[streamName] = status.DeepCopy()
}

func (c *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&PodLogStream{}).
		Watches(c.podSource, handler.Funcs{})

	return b, nil
}

type PodLogWatch struct {
	ctx    context.Context
	cancel func()

	streamName      types.NamespacedName
	podID           k8s.PodID
	namespace       k8s.Namespace
	cName           container.Name
	startWatchTime  time.Time
	terminationTime chan time.Time

	shouldPrefix bool // if true, we'll prefix logs with the container name
}

type podLogKey struct {
	streamName types.NamespacedName
	podID      k8s.PodID
	cID        container.ID
}
