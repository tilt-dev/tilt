package runtimelog

import (
	"context"
	"fmt"
	"io"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

// Collects logs from deployed containers.
type PodLogManager struct {
	kClient k8s.Client

	watches map[podLogKey]PodLogWatch
}

func NewPodLogManager(kClient k8s.Client) *PodLogManager {
	return &PodLogManager{
		kClient: kClient,
		watches: make(map[podLogKey]PodLogWatch),
	}
}

func cancelAll(watches []PodLogWatch) {
	for _, w := range watches {
		w.cancel()
	}
}

// Diff the current watches against the state store of what
// we're supposed to be watching, returning the changes
// we need to make.
func (m *PodLogManager) diff(ctx context.Context, st store.RStore) (setup []PodLogWatch, teardown []PodLogWatch) {
	state := st.RLockState()
	defer st.RUnlockState()

	// If we're not watching the FS for changes, then don't bother watching logs.
	if !state.WatchFiles {
		return nil, nil
	}

	stateWatches := make(map[podLogKey]bool)
	for _, ms := range state.ManifestStates() {
		runtime := ms.K8sRuntimeState()
		for _, pod := range runtime.PodList() {
			if !m.shouldWatchPodLogs(pod) {
				continue
			}

			// if pod has more than one container, we should prefix logs with the container name
			shouldPrefix := len(pod.Containers) > 1

			for _, c := range pod.Containers {
				// Key the log watcher by the container id, so we auto-restart the
				// watching if the container crashes.
				key := podLogKey{
					podID: pod.PodID,
					cID:   c.ID,
				}
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

func (m *PodLogManager) shouldWatchPodLogs(pod store.Pod) bool {
	if pod.PodID == "" || len(pod.Containers) == 0 {
		return false
	}

	// If an ID or name for the containers hasn't been created yet, weird things
	// will happen when we try to store them in the `m.watches` map.  This should
	// only happen if the pod is still in a weird creating state. It shouldn't
	// happen when user code is running.
	for _, container := range pod.Containers {
		if container.Name == "" || container.ID == "" {
			return false
		}
	}

	// Only try to fetch logs if pod is in a state that can handle it;
	// otherwise, it may reject our connection.
	if !(pod.Phase == v1.PodRunning || pod.Phase == v1.PodSucceeded ||
		pod.Phase == v1.PodFailed) {
		return false
	}

	return true
}

func (m *PodLogManager) OnChange(ctx context.Context, st store.RStore) {
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
		watch.terminationTime <- time.Now()
		watch.cancel()
	}()

	name := watch.name
	pID := watch.podID
	containerName := watch.cName
	ns := watch.namespace
	startTime := watch.startWatchTime
	ctx := logger.CtxWithLogHandler(watch.ctx, PodLogActionWriter{
		Store:        st,
		ManifestName: name,
		PodID:        pID,
	})
	if watch.shouldPrefix {
		prefix := fmt.Sprintf("[%s] ", watch.cName)
		ctx = logger.WithLogger(ctx, logger.NewPrefixedLogger(prefix, logger.Get(ctx)))
	}

	readCloser, err := m.kClient.ContainerLogs(ctx, pID, containerName, ns, startTime)
	if err != nil {
		// TODO(nick): Should this be Warnf/Errorf?
		logger.Get(ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
	defer func() {
		_ = readCloser.Close()
	}()

	_, err = io.Copy(logger.Get(ctx).Writer(logger.InfoLvl),
		NewHardCancelReader(ctx, readCloser))
	if err != nil && ctx.Err() == nil {
		// TODO(nick): Should this be Warnf/Errorf?
		logger.Get(ctx).Infof("Error streaming %s logs: %v", name, err)
		return
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
