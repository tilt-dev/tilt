package engine

import (
	"context"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerComposeEventWatcher struct {
	watching bool
	dcc      dockercompose.DockerComposeClient
}

func NewDockerComposeEventWatcher(dcc dockercompose.DockerComposeClient) *DockerComposeEventWatcher {
	return &DockerComposeEventWatcher{
		dcc: dcc,
	}
}

func (w *DockerComposeEventWatcher) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.WatchFiles && !w.watching
}

func (w *DockerComposeEventWatcher) OnChange(ctx context.Context, st store.RStore) {
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
		st.Dispatch(NewErrorAction(err))
		return
	}

	go dispatchDockerComposeEventLoop(ctx, ch, st)
}

func (w *DockerComposeEventWatcher) startWatch(ctx context.Context, configPath []string) (<-chan string, error) {
	return w.dcc.StreamEvents(ctx, configPath)
}

func dispatchDockerComposeEventLoop(ctx context.Context, ch <-chan string, st store.RStore) {
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

			st.Dispatch(DockerComposeEventAction{evt})
		case <-ctx.Done():
			return
		}
	}
}
