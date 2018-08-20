package engine

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/windmilleng/fsnotify"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
	"github.com/windmilleng/tilt/internal/watch"
)

// When we see a file change, wait this long to see if any other files have changed, and bundle all changes together.
// 200ms is not the result of any kind of research or experimentation
// it might end up being a significant part of deployment delay, if we get the total latency <2s
// it might also be long enough that it misses some changes if the user has some operation involving a large file
//   (e.g., a binary dependency in git), but that's hopefully less of a problem since we'd get it in the next build
const watchBufferMinRestInMs = 200

// When waiting for a `watchBufferDurationInMs`-long break in file modifications to aggregate notifications,
// if we haven't seen a break by the time `watchBufferMaxTimeInMs` has passed, just send off whatever we've got
const watchBufferMaxTimeInMs = 10000

var watchBufferMinRestDuration = watchBufferMinRestInMs * time.Millisecond
var watchBufferMaxDuration = watchBufferMaxTimeInMs * time.Millisecond

type Upper struct {
	b            BuildAndDeployer
	watcherMaker func() (watch.Notify, error)
	makeTimer    func(d time.Duration) <-chan time.Time
}

func NewUpper(manager service.Manager) (Upper, error) {
	b, err := NewLocalBuildAndDeployer(manager)
	if err != nil {
		return Upper{}, err
	}
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}
	return Upper{b, watcherMaker, time.After}, nil
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func (u Upper) coalesceEvents(eventChan <-chan fsnotify.Event) <-chan []fsnotify.Event {
	ret := make(chan []fsnotify.Event)
	go func() {
		defer close(ret)
		for {
			event, ok := <-eventChan
			if !ok {
				return
			}
			events := []fsnotify.Event{event}

			// keep grabbing changes until we've gone `watchBufferMinRestDuration` without seeing a change
			minRestTimer := u.makeTimer(watchBufferMinRestDuration)

			// but if we go too long before seeing a break (e.g., a process is constantly writing logs to that dir)
			// then just send what we've got
			timeout := u.makeTimer(watchBufferMaxDuration)

			done := false
			channelClosed := false
			for !done && !channelClosed {
				select {
				case event, ok := <-eventChan:
					if !ok {
						channelClosed = true
					} else {
						minRestTimer = u.makeTimer(watchBufferMinRestDuration)
						events = append(events, event)
					}
				case <-minRestTimer:
					done = true
				case <-timeout:
					done = true
				}
			}
			if len(events) > 0 {
				ret <- events
			}

			if channelClosed {
				return
			}
		}

	}()
	return ret
}

func (u Upper) Up(ctx context.Context, services []model.Service, watchMounts bool) error {
	var buildTokens []*buildToken
	for i := range services {
		buildToken, err := u.b.BuildAndDeploy(ctx, services[i], nil, nil)
		buildTokens = append(buildTokens, buildToken)
		if err != nil {
			return err
		}
	}

	if watchMounts {
		service := services[0]
		if len(services) > 1 {
			return errors.New("There is more than 1 service")
		}
		watcher, err := u.watcherMaker()
		if err != nil {
			return err
		}

		if len(service.Mounts) == 0 {
			return errors.New("service has 0 repos - nothing to watch")
		}

		for _, mount := range service.Mounts {
			err = watcher.Add(mount.Repo.LocalPath)
			if err != nil {
				return err
			}
		}

		coalescedEvents := u.coalesceEvents(watcher.Events())

		for {
			select {
			case err := <-watcher.Errors():
				return err
			case events, ok := <-coalescedEvents:
				if !ok {
					return nil
				}
				logger.Get(ctx).Infof("files changed, rebuilding %v", service.Name)

				var changedPaths []string
				for _, e := range events {
					path, err := filepath.Abs(e.Name)
					if err != nil {
						return err
					}
					changedPaths = append(changedPaths, path)
				}
				buildTokens[0], err = u.b.BuildAndDeploy(ctx, service, buildTokens[0], changedPaths)
				if err != nil {
					logger.Get(ctx).Infof("build failed: %v", err.Error())
				}

			}
		}
	}
	return nil
}
