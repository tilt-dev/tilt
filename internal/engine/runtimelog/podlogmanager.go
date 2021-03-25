package runtimelog

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

var podLogHealthCheck = 15 * time.Second
var podLogReconnectGap = 2 * time.Second

const IstioInitContainerName = container.Name("istio-init")
const IstioSidecarContainerName = container.Name("istio-proxy")

// Collects logs from deployed containers.
type PodLogManager struct {
	kClient k8s.Client

	watches         map[podLogKey]PodLogWatch
	hasClosedStream map[podLogKey]bool

	newTicker func(d time.Duration) *time.Ticker
	since     func(t time.Time) time.Duration
	now       func() time.Time
}

func NewPodLogManager(kClient k8s.Client) *PodLogManager {
	return &PodLogManager{
		kClient:         kClient,
		watches:         make(map[podLogKey]PodLogWatch),
		hasClosedStream: make(map[podLogKey]bool),
		newTicker:       time.NewTicker,
		since:           time.Since,
		now:             time.Now,
	}
}

// Always ignore the istio-init container.
// TODO(nick): Make this configurable. See https://github.com/tilt-dev/tilt/issues/3814
func (m *PodLogManager) initContainersWithWatchedLogs(pod store.Pod) []store.Container {
	result := []store.Container{}
	for _, c := range pod.InitContainers {
		if c.Name == IstioInitContainerName {
			continue
		}
		result = append(result, c)
	}
	return result
}

// Always ignore the istio-sidecar container.
// TODO(nick): Make this configurable. See https://github.com/tilt-dev/tilt/issues/3814
func (m *PodLogManager) runContainersWithWatchedLogs(pod store.Pod) []store.Container {
	result := []store.Container{}
	for _, c := range pod.Containers {
		if c.Name == IstioSidecarContainerName {
			continue
		}
		result = append(result, c)
	}
	return result

}

// Diff the current watches against the state store of what
// we're supposed to be watching, returning the changes
// we need to make.
func (m *PodLogManager) diff(ctx context.Context, st store.RStore) (setup []PodLogWatch, teardown []PodLogWatch) {
	state := st.RLockState()
	defer st.RUnlockState()

	stateWatches := make(map[podLogKey]bool)
	for _, mt := range state.Targets() {
		man := mt.Manifest

		// Skip logs that don't come from tiltfile-generated manifests
		// (in particular, the local metrics stack).
		if man.Source != model.ManifestSourceTiltfile {
			continue
		}

		ms := mt.State
		runtime := ms.K8sRuntimeState()
		for _, pod := range runtime.PodList() {
			if pod.PodID == "" {
				continue
			}

			initContainers := m.initContainersWithWatchedLogs(pod)
			runContainers := m.runContainersWithWatchedLogs(pod)
			containers := []store.Container{}
			containers = append(containers, initContainers...)
			containers = append(containers, runContainers...)

			for i, c := range containers {
				// Key the log watcher by the container id, so we auto-restart the
				// watching if the container crashes.
				key := podLogKey{
					podID: pod.PodID,
					cID:   c.ID,
				}
				if !m.shouldStreamContainerLogs(c, key) {
					continue
				}

				isInitContainer := i < len(initContainers)

				// We don't want to clutter the logs with a container name
				// if it's unambiguous what container we're looking at.
				//
				// Long-term, we should make the container name a log field
				// and have better ways to display it visually.
				shouldPrefix := isInitContainer || len(runContainers) > 1

				stateWatches[key] = true

				existing, isActive := m.watches[key]

				// Only stream logs that have happened since Tilt started.
				//
				// TODO(nick): We should really record when we started the `kubectl apply`,
				// and only stream logs since that happened.
				startWatchTime := state.TiltStartTime
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
					m.hasClosedStream[key] = true
					if c.Terminated {
						continue
					}
				}

				ctx, cancel := context.WithCancel(ctx)
				w := PodLogWatch{
					ctx:             ctx,
					cancel:          cancel,
					name:            ms.Name,
					podID:           pod.PodID,
					cName:           c.Name,
					namespace:       pod.Namespace,
					startWatchTime:  startWatchTime,
					terminationTime: make(chan time.Time, 1),
					shouldPrefix:    shouldPrefix,
				}
				m.watches[key] = w
				setup = append(setup, w)
			}
		}
	}

	for key, value := range m.watches {
		_, inState := stateWatches[key]
		if !inState {
			delete(m.watches, key)
			teardown = append(teardown, value)
		}
	}

	return setup, teardown
}

func (m *PodLogManager) shouldStreamContainerLogs(c store.Container, key podLogKey) bool {
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

func (m *PodLogManager) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if len(summary.Pods.Changes) == 0 {
		return
	}

	setup, teardown := m.diff(ctx, st)
	for _, watch := range teardown {
		watch.cancel()
	}

	for _, watch := range setup {
		go m.consumeLogs(watch, st)
	}
}

func (m *PodLogManager) consumeLogs(watch PodLogWatch, st store.RStore) {
	defer func() {
		watch.terminationTime <- m.now()
		watch.cancel()
	}()

	name := watch.name
	pID := watch.podID
	containerName := watch.cName
	ns := watch.namespace
	startReadTime := watch.startWatchTime
	ctx := logger.CtxWithLogHandler(watch.ctx, PodLogActionWriter{
		Store:        st,
		ManifestName: name,
		PodID:        pID,
	})
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
			logger.Get(ctx).Infof("Error streaming %s logs: %v", name, err)
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
			logger.Get(ctx).Infof("Error streaming %s logs: %v", name, err)
			return
		}
	}
}

type PodLogWatch struct {
	ctx    context.Context
	cancel func()

	name            model.ManifestName
	podID           k8s.PodID
	namespace       k8s.Namespace
	cName           container.Name
	startWatchTime  time.Time
	terminationTime chan time.Time

	shouldPrefix bool // if true, we'll prefix logs with the container name
}

type podLogKey struct {
	podID k8s.PodID
	cID   container.ID
}

type PodLogActionWriter struct {
	Store        store.RStore
	PodID        k8s.PodID
	ManifestName model.ManifestName
}

func (w PodLogActionWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.Store.Dispatch(store.NewLogAction(w.ManifestName, SpanIDForPod(w.PodID), level, fields, p))
	return nil
}

func SpanIDForPod(podID k8s.PodID) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("pod:%s", podID))
}

var _ store.Subscriber = &PodLogManager{}
