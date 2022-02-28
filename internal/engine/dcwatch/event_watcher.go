package dcwatch

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type EventWatcher struct {
	watching bool
	dcc      dockercompose.DockerComposeClient
	docker   docker.LocalClient
}

func NewEventWatcher(dcc dockercompose.DockerComposeClient, docker docker.LocalClient) *EventWatcher {
	return &EventWatcher{
		dcc:    dcc,
		docker: docker,
	}
}

func (w *EventWatcher) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	// TODO(nick): This should respond dynamically if the path changes.
	if summary.IsLogOnly() || w.watching {
		return nil
	}

	state := st.RLockState()
	project := state.DockerComposeProject()
	st.RUnlockState()

	if model.IsEmptyDockerComposeProject(project) {
		// No DC manifests to watch
		return nil
	}

	w.watching = true
	ch, err := w.startWatch(ctx, project)
	if err != nil {
		err = errors.Wrap(err, "Subscribing to docker-compose events")
		st.Dispatch(store.NewErrorAction(err))
		return nil
	}

	go w.dispatchEventLoop(ctx, ch, st)

	return nil
}

func (w *EventWatcher) startWatch(ctx context.Context, p v1alpha1.DockerComposeProject) (<-chan string, error) {
	return w.dcc.StreamEvents(ctx, p)
}

func (w *EventWatcher) dispatchEventLoop(ctx context.Context, ch <-chan string, st store.RStore) {
	for {
		select {
		case evtJson, ok := <-ch:
			if !ok {
				return
			}
			evt, err := dockercompose.EventFromJsonStr(evtJson)
			if err != nil {
				// TODO(maia): handle this error better?
				logger.Get(ctx).Debugf("[dcwatch] failed to unmarshal dc event '%s' with err: %v", evtJson, err)
				continue
			}

			if evt.Type != dockercompose.TypeContainer {
				continue
			}

			containerJSON, err := w.docker.ContainerInspect(ctx, evt.ID)
			if err != nil {
				logger.Get(ctx).Debugf("[dcwatch] inspecting container: %v", err)
				continue
			}

			if containerJSON.ContainerJSONBase == nil || containerJSON.ContainerJSONBase.State == nil {
				logger.Get(ctx).Debugf("[dcwatch] inspecting continer: no state found")
				continue
			}

			cState := containerJSON.ContainerJSONBase.State
			st.Dispatch(NewEventAction(evt, *cState))
		case <-ctx.Done():
			return
		}
	}
}

func NewEventAction(evt dockercompose.Event, state types.ContainerState) EventAction {
	return EventAction{
		Event:          evt,
		Time:           time.Now(),
		ContainerState: state,
	}
}
