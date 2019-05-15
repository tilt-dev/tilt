package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type EventWatchManager struct {
	kClient k8s.Client

	watches map[eventWatchKey]EventWatch
}

type EventWatch struct {
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

type eventWatchKey struct {
	podID k8s.PodID
	cID   container.ID
}

func NewEventWatchManager(kClient k8s.Client) *EventWatchManager {
	return &EventWatchManager{
		kClient: kClient,
		watches: make(map[eventWatchKey]EventWatch),
	}
}

func cancelAllEventWatches(watches []EventWatch) {
	for _, w := range watches {
		w.cancel()
	}
}

func (m *EventWatchManager) diff(ctx context.Context, st store.RStore) (setup []EventWatch, teardown []EventWatch) {
}

func (m *EventWatchManager) OnChange(ctx context.Context, st store.RStore) {
	_, teardown := m.diff(ctx, st)
	for _, watch := range teardown {
		watch.cancel()
	}

	m.kClient.
}
