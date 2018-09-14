package engine

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/watch"
)

type manifestFilesChangedEvent struct {
	manifest model.Manifest
	files    []string
}

type manifestWatcher struct {
	events <-chan manifestFilesChangedEvent
	errs   <-chan error
}

// returns a manifestWatcher that tells its reader when a manifest's file dependencies have changed
func makeManifestWatcher(
	ctx context.Context,
	watcherMaker watcherMaker,
	timerMaker timerMaker,
	manifests []model.Manifest) (*manifestWatcher, error) {

	var sns []manifestNotifyPair
	for _, manifest := range manifests {
		watcher, err := watcherMaker()
		if err != nil {
			return nil, err
		}

		if len(manifest.Mounts) == 0 {
			// no mounts -  nothing to watch
			continue
		}

		for _, mount := range manifest.Mounts {
			for _, p := range mount.Repo.LocalPaths {
				err = watcher.Add(p)
				if err != nil {
					return nil, err
				}
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

func addEventsToLeftover(
	leftover map[string]*manifestFilesChangedEvent,
	leftoverManifestOrder *[]string,
	events []manifestSingleFileChangeEvent) {
	for _, event := range events {
		if _, exists := leftover[string(event.manifest.Name)]; !exists {
			leftover[string(event.manifest.Name)] = &manifestFilesChangedEvent{event.manifest, []string{event.fileName}}
			// if we weren't already tracking this manifest name, stick it at the end of the order
			*leftoverManifestOrder = append(*leftoverManifestOrder, string(event.manifest.Name))
		} else {
			leftover[string(event.manifest.Name)].files = append(leftover[string(event.manifest.Name)].files, event.fileName)
		}
	}
}

type watchEventsStream struct {
	events <-chan manifestFilesChangedEvent
	errs   <-chan error
}

type manifestSingleFileChangeEvent struct {
	manifest model.Manifest
	fileName string
}

type manifestNotifyPair struct {
	manifest model.Manifest
	notify   watch.Notify
}

func makeFilter(ctx context.Context, manifest model.Manifest) (model.PathMatcher, error) {
	var repoRoots []string

	for _, mount := range manifest.Mounts {
		for _, p := range mount.Repo.LocalPaths {
			repoRoots = append(repoRoots, p)
		}
	}

	mrt, err := git.NewMultiRepoIgnoreTester(ctx, repoRoots)
	if err != nil {
		return nil, err
	}

	dit, err := dockerignore.NewMultiRepoDockerIgnoreTester(repoRoots)

	ci := model.NewCompositeMatcher([]model.PathMatcher{
		mrt,
		dit,
	})

	return ci, nil
}

// turns a list of (manifest, chan fsevent) pairs into a single chan (manifest, fsevent)
func snsToManifestWatcher(ctx context.Context, timerMaker timerMaker, sns []manifestNotifyPair) (*manifestWatcher, error) {
	events := make(chan manifestFilesChangedEvent)
	errs := make(chan error)

	for _, sn := range sns {
		coalescedEvents := coalesceEvents(timerMaker, sn.notify.Events())
		filter, err := makeFilter(ctx, sn.manifest)
		if err != nil {
			return nil, err
		}

		go func(manifest model.Manifest, watcher watch.Notify) {
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
					watchEvent := manifestFilesChangedEvent{manifest: manifest}
					for _, fe := range fsEvents {
						path, err := filepath.Abs(fe.Path)
						if err != nil {
							errs <- err
						}
						isIgnored, err := filter.Matches(path, false)
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
