package engine

import (
	"errors"
	"path/filepath"

	"github.com/windmilleng/fsnotify"
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

// returns a serviceWatcher that tells its reader when to try to start a service
func makeServiceWatcher(watcherMaker watcherMaker, timerMaker timerMaker, services []model.Service) (*serviceWatcher, error) {
	watchEventsStream, err := makeWatchEventsStream(watcherMaker, timerMaker, services)
	if err != nil {
		return nil, err
	}

	serviceChan := make(chan serviceFilesChangedEvent)
	errs := make(chan error)
	go func() {
		// a map of service names to a list of any unprocessed changes
		leftover := make(map[string]*serviceFilesChangedEvent)
		for {
			if len(leftover) == 0 {
				select {
				case events := <-watchEventsStream.events:
					addEventsToLeftover(leftover, events)
				case err := <-watchEventsStream.errs:
					errs <- err
					close(serviceChan)
					close(errs)
					return
				}
			} else {
				select {
				case events := <-watchEventsStream.events:
					addEventsToLeftover(leftover, events)
				case err := <-watchEventsStream.errs:
					errs <- err
					close(serviceChan)
					close(errs)
					return
				default:
				}
			}
			var serviceName string
			for i := range leftover {
				serviceName = i
				break
			}
			serviceChan <- *leftover[serviceName]
			delete(leftover, serviceName)
		}
	}()

	return &serviceWatcher{serviceChan, errs}, nil
}

func makeWatchEventsStream(watcherMaker watcherMaker, timerMaker timerMaker, services []model.Service) (*watchEventsStream, error) {
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

	return snsToServiceWatcher(timerMaker, sns), nil
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func coalesceEvents(timerMaker timerMaker, eventChan <-chan fsnotify.Event) <-chan []fsnotify.Event {
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

func addEventsToLeftover(leftover map[string]*serviceFilesChangedEvent, events []serviceSingleFileChangeEvent) {
	for _, event := range events {
		if _, exists := leftover[string(event.service.Name)]; !exists {
			leftover[string(event.service.Name)] = &serviceFilesChangedEvent{event.service, []string{event.fileName}}
		} else {
			leftover[string(event.service.Name)].files = append(leftover[string(event.service.Name)].files, event.fileName)
		}
	}
}

type watchEventsStream struct {
	events <-chan []serviceSingleFileChangeEvent
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

// turns a list of (service, chan fsevent) pairs into a single chan (service, fsevent)
func snsToServiceWatcher(timerMaker timerMaker, sns []serviceNotifyPair) *watchEventsStream {
	events := make(chan []serviceSingleFileChangeEvent)
	errs := make(chan error)

	for _, sn := range sns {
		coalescedEvents := coalesceEvents(timerMaker, sn.notify.Events())
		go func(service model.Service, watcher watch.Notify) {
			for {
				select {
				case err, ok := <-watcher.Errors():
					if !ok {
						close(events)
						close(errs)
						return
					}
					errs <- err
				case fsEvents, ok := <-coalescedEvents:
					if !ok {
						close(events)
						close(errs)
						return
					}
					var watchEvents []serviceSingleFileChangeEvent
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Name)
						if err != nil {
							errs <- err
						}
						watchEvents = append(watchEvents, serviceSingleFileChangeEvent{service, path})
					}
					events <- watchEvents
				}
			}
		}(sn.service, sn.notify)
	}

	return &watchEventsStream{events, errs}
}
