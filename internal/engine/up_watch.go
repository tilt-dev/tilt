package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

type serviceFilesChangedEvent struct {
	service model.Service
	files   []string
}

type serviceWatcher struct {
	events <-chan serviceFilesChangedEvent
	errs   <-chan error
}

// returns a serviceWatcher that tells its reader when a service's file dependencies have changed
func makeServiceWatcher(
	ctx context.Context,
	watcherMaker watcherMaker,
	timerMaker timerMaker,
	services []model.Service) (*serviceWatcher, error) {

	var sns []serviceNotifyPair
	for _, service := range services {
		watcher, err := watcherMaker()
		if err != nil {
			return nil, err
		}

		if len(service.Mounts) == 0 {
			// no mounts -  nothing to watch
			continue
		}

		for _, mount := range service.Mounts {
			err = watcher.Add(mount.Repo.LocalPath)
			if err != nil {
				return nil, err
			}
		}
		sns = append(sns, serviceNotifyPair{service, watcher})
	}

	if len(sns) == 0 {
		return nil, errors.New("--watch used when no services define mounts - nothing to watch")
	}

	ret, err := snsToServiceWatcher(ctx, timerMaker, sns)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func coalesceEvents(timerMaker timerMaker, eventChan <-chan watch.FileEvent) <-chan []watch.FileEvent {
	ret := make(chan []watch.FileEvent)
	go func() {
		defer close(ret)

		for {
			event, ok := <-eventChan
			if !ok {
				return
			}
			events := []watch.FileEvent{event}

			// keep grabbing changes until we've gone `watchBufferMinRestDuration` without seeing a change
			minRestTimer := timerMaker(watchBufferMinRestDuration)

			// but if we go too long before seeing a break (e.g., a process is constantly writing logs to that dir)
			// then just send what we've got
			timeout := timerMaker(watchBufferMaxDuration)

			done := false
			channelClosed := false
			for !done && !channelClosed {
				select {
				case event, ok := <-eventChan:
					if !ok {
						channelClosed = true
					} else {
						minRestTimer = timerMaker(watchBufferMinRestDuration)
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

func addEventsToLeftover(
	leftover map[string]*serviceFilesChangedEvent,
	leftoverServiceOrder *[]string,
	events []serviceSingleFileChangeEvent) {
	for _, event := range events {
		if _, exists := leftover[string(event.service.Name)]; !exists {
			leftover[string(event.service.Name)] = &serviceFilesChangedEvent{event.service, []string{event.fileName}}
			// if we weren't already tracking this service name, stick it at the end of the order
			*leftoverServiceOrder = append(*leftoverServiceOrder, string(event.service.Name))
		} else {
			leftover[string(event.service.Name)].files = append(leftover[string(event.service.Name)].files, event.fileName)
		}
	}
}

type watchEventsStream struct {
	events <-chan serviceFilesChangedEvent
	errs   <-chan error
}

type serviceSingleFileChangeEvent struct {
	service  model.Service
	fileName string
}

type serviceNotifyPair struct {
	service model.Service
	notify  watch.Notify
}

func makeFilter(ctx context.Context, service model.Service) (ignore.Tester, error) {
	var repoRoots []string

	for _, mount := range service.Mounts {
		repoRoots = append(repoRoots, mount.Repo.LocalPath)
	}

	eventFilter, err := git.NewMultiRepoIgnoreTester(ctx, repoRoots)
	if err != nil {
		return nil, err
	}

	return eventFilter, nil
}

// turns a list of (service, chan fsevent) pairs into a single chan (service, fsevent)
func snsToServiceWatcher(ctx context.Context, timerMaker timerMaker, sns []serviceNotifyPair) (*serviceWatcher, error) {
	events := make(chan serviceFilesChangedEvent)
	errs := make(chan error)

	for _, sn := range sns {
		coalescedEvents := coalesceEvents(timerMaker, sn.notify.Events())
		filter, err := makeFilter(ctx, sn.service)
		if err != nil {
			return nil, err
		}

		go func(service model.Service, watcher watch.Notify) {
			// TODO(matt) this will panic if we actually close channels. look at "merge" in https://blog.golang.org/pipelines
			//defer close(events)
			//defer close(errs)

			for {
				select {
				case err, ok := <-watcher.Errors():
					if !ok {
						return
					}
					errs <- err
				case fsEvents, ok := <-coalescedEvents:
					if !ok {
						return
					}
					watchEvent := serviceFilesChangedEvent{service: service}
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Path)
						if err != nil {
							errs <- err
						}
						isIgnored, err := filter.IsIgnored(path, false)
						if !isIgnored {
							watchEvent.files = append(watchEvent.files, path)
						}
					}
					if len(watchEvent.files) > 0 {
						events <- watchEvent
					}
				}
			}
		}(sn.service, sn.notify)
	}

	return &serviceWatcher{events, errs}, nil
}
