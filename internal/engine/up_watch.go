package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

type manifestFilesChangedAction struct {
	manifestName model.ManifestName
	files        []string
}

func (manifestFilesChangedAction) Action() {}

// returns a manifestWatcher that tells its reader when a manifest's file dependencies have changed
func makeManifestWatcher(
	ctx context.Context,
	st *store.Store,
	watcherMaker fsWatcherMaker,
	timerMaker timerMaker,
	manifests []model.Manifest) error {

	var sns []manifestNotifyPair
	for _, manifest := range manifests {
		watcher, err := watcherMaker()
		if err != nil {
			return err
		}

		localPaths := manifest.LocalPaths()
		if len(localPaths) == 0 {
			// no mounts -  nothing to watch
			continue
		}

		for _, localPath := range localPaths {
			err = watcher.Add(localPath)
			if err != nil {
				return err
			}
		}

		for _, cf := range manifest.ConfigFiles {
			err = watcher.Add(cf)
			if err != nil {
				return err
			}
		}

		sns = append(sns, manifestNotifyPair{manifest, watcher})
	}

	if len(sns) == 0 {
		return errors.New("--watch used when no manifests define mounts - nothing to watch")
	}

	return snsToManifestWatcher(ctx, st, timerMaker, sns)
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
func snsToManifestWatcher(ctx context.Context, st *store.Store, timerMaker timerMaker, sns []manifestNotifyPair) error {
	for _, sn := range sns {
		coalescedEvents := coalesceEvents(timerMaker, sn.notify.Events())

		go func(manifest model.Manifest, watcher watch.Notify) {
			// TODO(matt) this will panic if we actually close channels. look at "merge" in https://blog.golang.org/pipelines
			//defer close(events)
			//defer close(errs)

			filter := ignore.CreateFileChangeFilter(manifest)
			configMatcher, err := manifest.ConfigMatcher()
			if err != nil {
				logger.Get(ctx).Infof("Error getting ConfigMatcher: %v", err)
			}
			for {
				select {
				case err, ok := <-watcher.Errors():
					if !ok {
						return
					}
					st.Dispatch(NewErrorAction(err))

				case fsEvents, ok := <-coalescedEvents:
					if !ok {
						return
					}
					watchEvent := manifestFilesChangedAction{manifestName: manifest.Name}
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Path)
						if err != nil {
							st.Dispatch(NewErrorAction(err))
							continue
						}
						isIgnored, err := filter.Matches(path, false)
						if err != nil {
							st.Dispatch(NewErrorAction(err))
							continue
						}
						isConfig, err := configMatcher.Matches(path, false)
						if err != nil {
							st.Dispatch(NewErrorAction(err))
							continue
						}
						if isConfig {
							watchEvent.files = append(watchEvent.files, path)
						} else if !isIgnored {
							watchEvent.files = append(watchEvent.files, path)
						}
					}
					if len(watchEvent.files) > 0 {
						st.Dispatch(watchEvent)
					}
				}
			}
		}(sn.manifest, sn.notify)
	}

	return nil
}
