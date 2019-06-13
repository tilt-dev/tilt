package engine

import (
	"context"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
)

type EventWatchManager struct {
	kClient  k8s.Client
	watching bool
}

func NewEventWatchManager(kClient k8s.Client) *EventWatchManager {
	return &EventWatchManager{
		kClient: kClient,
	}
}

func (m *EventWatchManager) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8s := false
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8s = true
		}
	}
	return atLeastOneK8s && state.WatchFiles && !m.watching
}

func (m *EventWatchManager) OnChange(ctx context.Context, st store.RStore) {
	if !m.needsWatch(st) {
		return
	}

	m.watching = true

	ch, err := m.kClient.WatchEvents(ctx)
	if err != nil {
		err = errors.Wrap(err, "Error watching k8s events\n")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go m.dispatchEventsLoop(ctx, ch, st)
}

func (m *EventWatchManager) dispatchEventsLoop(ctx context.Context, ch <-chan *v1.Event, st store.RStore) {
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}

			st.Dispatch(store.NewK8sEventAction(event))
		case <-ctx.Done():
			return
		}
	}
}
