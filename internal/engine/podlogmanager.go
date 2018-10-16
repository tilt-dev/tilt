package engine

import (
	"context"
	"fmt"
	"io"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/internal/store"
	"k8s.io/api/core/v1"
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

// Diff the current watches against the state store of what
// we're supposed to be watching, returning the changes
// we need to make.
func (m *PodLogManager) diff(ctx context.Context, st *store.Store) (setup []PodLogWatch, teardown []PodLogWatch) {
	state := st.RLockState()
	defer st.RUnlockState()

	// If we're not watching the mounts, then don't bother watching logs.
	if !state.WatchMounts {
		return nil, nil
	}

	stateWatches := make(map[podLogKey]bool, 0)
	for _, ms := range state.ManifestStates {
		pod := ms.Pod
		if pod.PodID == "" || pod.ContainerName == "" || pod.ContainerID == "" {
			continue
		}

		// Only fetch pod logs if the pod is running.
		// Otherwise it will reject our connection.
		if ms.Pod.Phase != v1.PodRunning {
			continue
		}

		// Key the log watcher by the container id, so we auto-restart the
		// watching if the container crashes.
		key := podLogKey{
			podID: pod.PodID,
			cID:   pod.ContainerID,
		}
		stateWatches[key] = true

		_, isActive := m.watches[key]
		if isActive {
			continue
		}

		ctx, cancel := context.WithCancel(ctx)
		w := PodLogWatch{
			ctx:       ctx,
			cancel:    cancel,
			name:      ms.Manifest.Name,
			podID:     pod.PodID,
			cID:       pod.ContainerID,
			cName:     pod.ContainerName,
			namespace: pod.Namespace,
		}
		m.watches[key] = w
		setup = append(setup, w)
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

func (m *PodLogManager) OnChange(ctx context.Context, st *store.Store) {
	setup, teardown := m.diff(ctx, st)
	for _, watch := range teardown {
		watch.cancel()
	}

	for _, watch := range setup {
		go m.consumeLogs(watch, st)
	}
}

func (m *PodLogManager) consumeLogs(watch PodLogWatch, st *store.Store) {
	name := watch.name
	pID := watch.podID
	containerName := watch.cName
	ns := watch.namespace
	readCloser, err := m.kClient.ContainerLogs(watch.ctx, pID, containerName, ns)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
	defer func() {
		_ = readCloser.Close()
	}()

	logWriter := logger.Get(watch.ctx).Writer(logger.InfoLvl)
	prefixLogWriter := output.NewPrefixedWriter(fmt.Sprintf("[%s] ", name), logWriter)
	actionWriter := PodLogActionWriter{
		store:        st,
		manifestName: name,
		podID:        pID,
	}
	multiWriter := io.MultiWriter(prefixLogWriter, actionWriter)

	_, err = io.Copy(multiWriter, readCloser)
	if err != nil {
		logger.Get(watch.ctx).Infof("Error streaming %s logs: %v", name, err)
		return
	}
}

type PodLogWatch struct {
	ctx    context.Context
	cancel func()

	name      model.ManifestName
	podID     k8s.PodID
	namespace k8s.Namespace
	cID       k8s.ContainerID
	cName     k8s.ContainerName
}

type podLogKey struct {
	podID k8s.PodID
	cID   k8s.ContainerID
}

type PodLogActionWriter struct {
	store        *store.Store
	podID        k8s.PodID
	manifestName model.ManifestName
}

func (w PodLogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(PodLogAction{
		PodID:        w.podID,
		ManifestName: w.manifestName,
		Log:          append([]byte{}, p...),
	})
	return len(p), nil
}

var _ store.Subscriber = &PodLogManager{}
