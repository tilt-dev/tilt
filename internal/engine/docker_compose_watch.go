package engine

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"

	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

type DockerComposeWatcher struct {
	watching bool
}

func NewDockerComposeWatcher() *DockerComposeWatcher {
	fmt.Println("making a docker-compose watcher hoorayyyy")
	return &DockerComposeWatcher{}
}

func (w *DockerComposeWatcher) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.WatchMounts && !w.watching
}

func (w *DockerComposeWatcher) OnChange(ctx context.Context, st store.RStore) {
	if !w.needsWatch(st) {
		return
	}
	w.watching = true

	state := st.RLockState()
	configPath := state.DockerComposeYAMLPath()
	st.RUnlockState()

	if configPath == "" {
		// No DC manifests to watch
		return
	}

	ch, err := w.startWatch(ctx, configPath)
	if err != nil {
		err = errors.Wrap(err, "Subscribing to docker-compose events")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go dispatchDockerComposeEventLoop(ctx, ch, st)
}

func (w *DockerComposeWatcher) startWatch(ctx context.Context, configPath string) (<-chan string, error) {
	fmt.Printf("here's a watcher for docker yaml: %s\n", configPath)

	ch := make(chan string)

	cmd := exec.CommandContext(ctx, "docker-compose", "-f", configPath, "events", "--json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ch, errors.Wrap(err, "making stdout pipe for `docker-compose events")
	}

	go func() {
		cmd.Start()

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			ch <- scanner.Text()
		}

		if err := scanner.Err(); err != nil {
			logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] scanning `events` output: %v", err)
		}

		err = cmd.Wait()
		if err != nil {
			logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] exited with error: %v", err)
		}
	}()

	return ch, nil
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
				// ~~ what do i do with this error?
				logger.Get(ctx).Infof("[DOCKER-COMPOSE WATCHER] failed to unmarshal dc event '%s' with err: %v", evtJson, err)
				continue
			}
			spew.Dump(evt)
			// ~~and then dispatch... something.
			// st.Dispatch(NewPodChangeAction(pod))
		case <-ctx.Done():
			return
		}
	}
}
