package engine

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
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

	// If we're not watching the mounts, then don't bother watching logs.
	if !state.WatchMounts {
		return nil, nil
	}

	stateWatches := make(map[podLogKey]bool)
	for _, ms := range state.ManifestStates() {
		for _, pod := range ms.PodSet.PodList() {
			if pod.PodID == "" {
				continue
			}

			if pod.ContainerName == "" || pod.ContainerID == "" {
				continue
			}

			// Only fetch pod logs if the pod is running.
			// Otherwise it will reject our connection.
			if pod.Phase != v1.PodRunning {
				continue
			}

			// NOTE(maia): setting up logWatchers using both containerInfos and pod.ContainerName etc.
			// is a temporary hack. Put this in for backwards compatibility.
			containerInfos := pod.ContainerInfos
			if len(containerInfos) == 0 {
				containerInfos = []store.ContainerInfo{
					store.ContainerInfo{ID: pod.ContainerID, Name: pod.ContainerName},
				}
			}
			// if pod has more than one container, we should prefix logs with the container name
			shouldPrefix := len(containerInfos) > 1

			for _, cInfo := range containerInfos {
				// Key the log watcher by the container id, so we auto-restart the
				// watching if the container crashes.
				key := podLogKey{
					podID: pod.PodID,
					cID:   cInfo.ID,
				}
				stateWatches[key] = true

				existing, isActive := m.watches[key]
				startWatchTime := time.Unix(0, 0)
				if isActive {
					if existing.ctx.Err() == nil {
						// The active pod watcher is still tailing the logs,
						// nothing to do.
						continue
					}

					// The active pod watcher got cancelled somehow,
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
					cName:           cInfo.Name,
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
	readCloser, err := m.kClient.ContainerLogs(watch.ctx, pID, containerName, ns, startTime)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
	defer func() {
		_ = readCloser.Close()
	}()

	logWriter := logger.Get(watch.ctx).Writer(logger.InfoLvl)
	prefix := logPrefix(name.String())
	prefixLogWriter := logger.NewPrefixedWriter(prefix, logWriter)
	actionWriter := PodLogActionWriter{
		store:        st,
		manifestName: name,
		podID:        pID,
	}
	multiWriter := io.MultiWriter(prefixLogWriter, actionWriter)
	if watch.shouldPrefix {
		prefix = fmt.Sprintf("[%s] ", watch.cName)
		multiWriter = logger.NewPrefixedWriter(prefix, multiWriter)
	}

	_, err = io.Copy(multiWriter, NewHardCancelReader(watch.ctx, readCloser))
	if err != nil && watch.ctx.Err() == nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
}

func logPrefix(n string) string {
	max := 12
	spaces := ""
	if len(n) > max {
		n = n[:max-1] + "…"
	} else {
		spaces = strings.Repeat(" ", max-len(n))
	}
	return fmt.Sprintf("%s%s┊ ", n, spaces)
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
	store        store.RStore
	podID        k8s.PodID
	manifestName model.ManifestName
}

func (w PodLogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(PodLogAction{
		PodID:        w.podID,
		ManifestName: w.manifestName,
		logEvent:     newLogEvent(append([]byte{}, p...)),
	})
	return len(p), nil
}

var _ store.Subscriber = &PodLogManager{}
