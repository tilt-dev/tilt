package engine

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/windmilleng/fsnotify"
	"github.com/windmilleng/tilt/internal/k8s"
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

// TODO(nick): maybe this should be called 'BuildEngine' or something?
// Upper seems like a poor and undescriptive name.
type Upper struct {
	b            BuildAndDeployer
	watcherMaker func() (watch.Notify, error)
	makeTimer    func(d time.Duration) <-chan time.Time
}

type watchEvent struct {
	svcName  string
	fileName string
}

func NewUpper(ctx context.Context, manager service.Manager, env k8s.Env) (Upper, error) {
	b, err := NewLocalBuildAndDeployer(ctx, manager, env)
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

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-Up")
	defer span.Finish()

	var watcher watch.Notify
	buildTokens := make(map[string]*buildToken)
	servicesByName := make(map[string]model.Service)
	var watchEvents <-chan []watchEvent
	var errs <-chan error

	if watchMounts {
		var err error
		watchers := make(map[string]watch.Notify)
		for _, service := range services {

			watcher, err = u.watcherMaker()
			if err != nil {
				return err
			}

			if len(services[0].Mounts) == 0 {
				return errors.New("service has 0 repos - nothing to watch")
			}

			for _, mount := range services[0].Mounts {
				err = watcher.Add(mount.Repo.LocalPath)
				if err != nil {
					return err
				}
			}
			watchers[string(service.Name)] = watcher
		}
		watchEvents, errs = u.combineWatchers(watchers)
	}
	for i := range services {
		buildToken, err := u.b.BuildAndDeploy(ctx, services[i], nil, nil)
		buildTokens[string(services[i].Name)] = buildToken
		servicesByName[string(services[i].Name)] = services[i]
		if err != nil {
			return err
		}
	}

	if watchMounts {

		leftover := make(map[string][]string)
		for {
			if len(leftover) == 0 {
				events := <-watchEvents
				for _, event := range events {
					leftover[event.svcName] = append(leftover[event.svcName], event.fileName)
				}
			} else {
				select {
				case events := <-watchEvents:
					for _, event := range events {
						leftover[event.svcName] = append(leftover[event.svcName], event.fileName)
					}
				case err := <-errs:
					return err
				default:
				}
			}
			var serviceName string
			for i := range leftover {
				serviceName = i
				break
			}
			var err error
			logger.Get(ctx).Infof("files changed, rebuilding %v", serviceName)
			buildTokens[serviceName], err = u.b.BuildAndDeploy(ctx, servicesByName[serviceName], buildTokens[serviceName], leftover[serviceName])
			if err != nil {
				logger.Get(ctx).Infof("build failed: %v", err.Error())
			}
			delete(leftover, serviceName)
		}
	}
	return nil
}

func (u Upper) combineWatchers(watchers map[string]watch.Notify) (<-chan []watchEvent, <-chan error) {
	events := make(chan []watchEvent)
	errs := make(chan error)

	for s, watcher := range watchers {
		coalescedEvents := u.coalesceEvents(watcher.Events())
		go func(s string, watcher watch.Notify) {
			for {
				select {
				case err := <-watcher.Errors():
					errs <- err
				case fsEvents, ok := <-coalescedEvents:
					if !ok {
						close(events)
						close(errs)
						return
					}
					var watchEvents []watchEvent
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Name)
						if err != nil {
							errs <- err
						}
						watchEvents = append(watchEvents, watchEvent{s, path})
					}
					events <- watchEvents
				}
			}
		}(s, watcher)
	}

	return events, errs
}
