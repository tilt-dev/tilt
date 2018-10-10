package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

type manifestFilesChangedEvent struct {
	manifestName model.ManifestName
	files        []string
}

type manifestWatcher struct {
	events <-chan manifestFilesChangedEvent
	errs   <-chan error
}

func newDummyManifestWatcher() *manifestWatcher {
	return &manifestWatcher{}
}

// returns a manifestWatcher that tells its reader when a manifest's file dependencies have changed
func makeManifestWatcher(
	ctx context.Context,
	watcherMaker fsWatcherMaker,
	timerMaker timerMaker,
	manifests []model.Manifest) (*manifestWatcher, error) {

	var sns []manifestNotifyPair
	for _, manifest := range manifests {
		watcher, err := watcherMaker()
		if err != nil {
			return nil, err
		}

		localPaths := manifest.LocalPaths()
		if len(localPaths) == 0 {
			// no mounts -  nothing to watch
			continue
		}

		for _, localPath := range localPaths {
			err = watcher.Add(localPath)
			if err != nil {
				return nil, err
			}
		}

		for _, cf := range manifest.ConfigFiles {
			err = watcher.Add(cf)
			if err != nil {
				return nil, err
			}
		}

		sns = append(sns, manifestNotifyPair{manifest, watcher})
	}

	if len(sns) == 0 {
		return nil, errors.New("--watch used when no manifests define mounts - nothing to watch")
	}

	ret, err := snsToManifestWatcher(ctx, timerMaker, sns)
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

type manifestNotifyPair struct {
	manifest model.Manifest
	notify   watch.Notify
}

// turns a list of (manifest, chan fsevent) pairs into a single chan (manifest, fsevent)
func snsToManifestWatcher(ctx context.Context, timerMaker timerMaker, sns []manifestNotifyPair) (*manifestWatcher, error) {
	events := make(chan manifestFilesChangedEvent)
	errs := make(chan error)

	for _, sn := range sns {
		coalescedEvents := coalesceEvents(timerMaker, sn.notify.Events())

		go func(manifest model.Manifest, watcher watch.Notify) {
			// TODO(matt) this will panic if we actually close channels. look at "merge" in https://blog.golang.org/pipelines
			//defer close(events)
			//defer close(errs)

			filter := ignore.CreateFileChangeFilter(manifest)
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
					watchEvent := manifestFilesChangedEvent{manifestName: manifest.Name}
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Path)
						if err != nil {
							errs <- err
						}
						isIgnored, err := filter.Matches(path, false)
						if err != nil {
							errs <- err
						}
						if !isIgnored {
							watchEvent.files = append(watchEvent.files, path)
						}
					}
					if len(watchEvent.files) > 0 {
						events <- watchEvent
					}
				}
			}
		}(sn.manifest, sn.notify)
	}

	return &manifestWatcher{events, errs}, nil
}
