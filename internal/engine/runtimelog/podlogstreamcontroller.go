package runtimelog

import (
	"context"
	"fmt"
	"io"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
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
type PodLogStreamController struct {
	client  ctrlclient.Client
	st      store.RStore
	kClient k8s.Client

	watches         map[podLogKey]PodLogWatch
	hasClosedStream map[podLogKey]bool

	newTicker func(d time.Duration) *time.Ticker
	since     func(t time.Time) time.Duration
	now       func() time.Time
}

func NewPodLogStreamController(client ctrlclient.Client, st store.RStore, kClient k8s.Client) *PodLogStreamController {
	return &PodLogStreamController{
		client:          client,
		st:              st,
		kClient:         kClient,
		watches:         make(map[podLogKey]PodLogWatch),
		hasClosedStream: make(map[podLogKey]bool),
		newTicker:       time.NewTicker,
		since:           time.Since,
		now:             time.Now,
	}
}

// Filter containers based on the inclusions/exclusions in the PodLogStream spec.
func (m *PodLogStreamController) filterContainers(stream *PodLogStream, containers []store.Container) []store.Container {
	if len(stream.Spec.OnlyContainers) > 0 {
		only := make(map[container.Name]bool, len(stream.Spec.OnlyContainers))
		for _, name := range stream.Spec.OnlyContainers {
			only[container.Name(name)] = true
		}

		result := []store.Container{}
		for _, c := range containers {
			if only[c.Name] {
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

		result := []store.Container{}
		for _, c := range containers {
			if !ignore[c.Name] {
				result = append(result, c)
			}
		}
		return result
	}
	return containers
}

// Determine which PodLogStreams to reconcile, and all the Pods in the engine state.
//
// Currently grabs all the PodLogStreams from the EngineStore.
// When we switch to the reconciler API, the apiserver infrastructure
// will do this for us and we'll fetch our own Pods.
func (c *PodLogStreamController) toReconcile(ctx context.Context, st store.RStore) map[string]*PodLogStream {
	state := st.RLockState()
	defer st.RUnlockState()

	result := make(map[string]*PodLogStream)
	for k, v := range state.PodLogStreams {
		result[k] = v
	}

	for k := range c.watches {
		if _, ok := result[k.streamName]; !ok {
			// Record that we're currently streaming from a pod,
			// but the API object has been deleted.
			result[k.streamName] = nil
		}
	}

	return result
}

func (m *PodLogStreamController) shouldStreamContainerLogs(c store.Container, key podLogKey) bool {
	if c.ID == "" {
		return false
	}

	if c.Terminated && m.hasClosedStream[key] {
		return false
	}

	if !(c.Running || c.Terminated) {
		return false
	}

	return true

}

func (c *PodLogStreamController) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if len(summary.Pods.Changes) == 0 && len(summary.PodLogStreams.Changes) == 0 {
		return
	}

	reconcileMap := c.toReconcile(ctx, st)
	for k, v := range reconcileMap {
		c.reconcile(ctx, st, k, v)
	}
}

// Reconcile the given stream against what we're currently tracking.
func (r *PodLogStreamController) reconcile(ctx context.Context, st store.RStore, streamName string, stream *PodLogStream) {
	if stream == nil {
		r.deleteStreams(streamName)
		return
	}

	ctx = store.MustObjectLogHandler(ctx, r.st, stream)
	podNN := types.NamespacedName{Name: stream.Spec.Pod, Namespace: stream.Spec.Namespace}
	pod, err := r.kClient.PodFromInformerCache(ctx, podNN)
	if (err != nil && apierrors.IsNotFound(err)) ||
		(pod != nil && pod.DeletionTimestamp != nil && !pod.DeletionTimestamp.IsZero()) {
		r.deleteStreams(streamName)
		return
	} else if err != nil {
		logger.Get(ctx).Debugf("streaming logs: %v", err)
		return
	} else if pod == nil {
		logger.Get(ctx).Debugf("streaming logs: pod not found: %s", podNN)
		return
	}

	initContainers := r.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.InitContainerStatuses))
	runContainers := r.filterContainers(stream, k8sconv.PodContainers(ctx, pod, pod.Status.ContainerStatuses))
	containers := []store.Container{}
	containers = append(containers, initContainers...)
	containers = append(containers, runContainers...)

	containerWatches := make(map[podLogKey]bool)
	for i, c := range containers {
		// Key the log watcher by the container id, so we auto-restart the
		// watching if the container crashes.
		key := podLogKey{
			streamName: streamName,
			podID:      k8s.PodID(podNN.Name),
			cID:        c.ID,
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
			if c.Terminated {
				continue
			}
		}

		ctx, cancel := context.WithCancel(ctx)
		w := PodLogWatch{
			ctx:             ctx,
			cancel:          cancel,
			podID:           k8s.PodID(podNN.Name),
			cName:           c.Name,
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
}

// Delete all the streams generated by the named API object
func (c *PodLogStreamController) deleteStreams(streamName string) {
	for k, watch := range c.watches {
		if k.streamName != streamName {
			continue
		}
		watch.cancel()
		delete(c.watches, k)
	}
}

func (m *PodLogStreamController) consumeLogs(watch PodLogWatch, st store.RStore) {
	defer func() {
		watch.terminationTime <- m.now()
		watch.cancel()
	}()

	pID := watch.podID
	containerName := watch.cName
	ns := watch.namespace
	startReadTime := watch.startWatchTime
	ctx := watch.ctx
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

			// TODO(nick): Should this be Warnf/Errorf?
			logger.Get(ctx).Infof("Error streaming %s logs: %v", pID, err)
			return
		}

		reader := NewHardCancelReader(ctx, readCloser)
		reader.now = m.now

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

		_, err = io.Copy(logger.Get(ctx).Writer(logger.InfoLvl), reader)
		_ = readCloser.Close()
		close(done)
		cancel()

		if !retry && err != nil && ctx.Err() == nil {
			// TODO(nick): Should this be Warnf/Errorf?
			logger.Get(ctx).Infof("Error streaming %s logs: %v", pID, err)
			return
		}
	}
}

type PodLogWatch struct {
	ctx    context.Context
	cancel func()

	podID           k8s.PodID
	namespace       k8s.Namespace
	cName           container.Name
	startWatchTime  time.Time
	terminationTime chan time.Time

	shouldPrefix bool // if true, we'll prefix logs with the container name
}

type podLogKey struct {
	streamName string
	podID      k8s.PodID
	cID        container.ID
}

var _ store.Subscriber = &PodLogStreamController{}
