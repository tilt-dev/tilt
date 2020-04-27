package dcwatch

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type EventWatcher struct {
	watching bool
	dcc      dockercompose.DockerComposeClient
}

func NewEventWatcher(dcc dockercompose.DockerComposeClient) *EventWatcher {
	return &EventWatcher{
		dcc: dcc,
	}
}

func (w *EventWatcher) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.EngineMode.WatchesRuntime() && !w.watching
}

func (w *EventWatcher) OnChange(ctx context.Context, st store.RStore) {
	if !w.needsWatch(st) {
		return
	}

	state := st.RLockState()
	configPaths := state.DockerComposeConfigPath()
	st.RUnlockState()

	if len(configPaths) == 0 {
		// No DC manifests to watch
		return
	}

	w.watching = true
	ch, err := w.startWatch(ctx, configPaths)
	if err != nil {
		err = errors.Wrap(err, "Subscribing to docker-compose events")
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	go dispatchEventLoop(ctx, ch, st)
}

func (w *EventWatcher) startWatch(ctx context.Context, configPath []string) (<-chan string, error) {
	return w.dcc.StreamEvents(ctx, configPath)
}

func dispatchEventLoop(ctx context.Context, ch <-chan string, st store.RStore) {
	for {
		select {
		case evtJson, ok := <-ch:
			if !ok {
				return
			}
			evt, err := dockercompose.EventFromJsonStr(evtJson)
			if err != nil {
				// TODO(maia): handle this error better?
				logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] failed to unmarshal dc event '%s' with err: %v", evtJson, err)
				continue
			}

			st.Dispatch(NewEventAction(evt))
		case <-ctx.Done():
			return
		}
	}
}

func NewEventAction(evt dockercompose.Event) EventAction {
	return EventAction{
		Event: evt,
		Time:  time.Now(),
	}
}
