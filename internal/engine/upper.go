package engine

import (
	"context"
	"errors"
	"io"
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
const watchBufferDurationInMs = 200

type Upper struct {
	b            BuildAndDeployer
	watcherMaker func() (watch.Notify, error)
	sleeper      func(d time.Duration)
}

func NewUpper(manager service.Manager) (Upper, error) {
	b, err := NewLocalBuildAndDeployer(manager)
	if err != nil {
		return Upper{}, err
	}
	watcherMaker := func() (watch.Notify, error) {
		return watch.NewWatcher()
	}
	sleeper := time.Sleep
	return Upper{b, watcherMaker, sleeper}, nil
}

//makes an attempt to read some events from `eventChan` so that multiple file changes that happen at the same time
//from the user's perspective are grouped together.
func (u Upper) coalesceEvents(eventChan chan fsnotify.Event) []fsnotify.Event {
	var events []fsnotify.Event
	doneWaitingForChanges := false
	for !doneWaitingForChanges {
		// keep grabbing changes until we've gone `watchBufferDurationInMs` without seeing a change
		u.sleeper(watchBufferDurationInMs * time.Millisecond)
		filesHaveChanged := false
		haveReadAllChangesSinceTimeout := false
		for !haveReadAllChangesSinceTimeout {
			select {
			case event := <-eventChan:
				events = append(events, event)
				filesHaveChanged = true
			default:
				haveReadAllChangesSinceTimeout = true
			}
		}
		if !filesHaveChanged {
			doneWaitingForChanges = true
		}
	}
	return events
}

func (u Upper) Up(ctx context.Context, service model.Service, watchMounts bool, stdout io.Writer, stderr io.Writer) error {
	buildToken, err := u.b.BuildAndDeploy(ctx, service, nil, nil)
	if err != nil {
		return err
	}

	if watchMounts {
		watcher, err := u.watcherMaker()
		if err != nil {
			return err
		}

		if len(service.Mounts) == 0 {
			return errors.New("service has 0 repos - nothing to watch")
		}

		for _, mount := range service.Mounts {
			watcher.Add(mount.Repo.LocalPath)
		}

		for {
			select {
			case err := <-watcher.Errors():
				return err
			case event := <-watcher.Events():
				logger.Get(ctx).Info("files changed, rebuilding %v", service.Name)
				events := []fsnotify.Event{event}
				events = append(events, u.coalesceEvents(watcher.Events())...)

				var changedPaths []string
				for _, e := range events {
					path, err := filepath.Abs(e.Name)
					if err != nil {
						return err
					}
					changedPaths = append(changedPaths, path)
				}
				buildToken, err = u.b.BuildAndDeploy(ctx, service, buildToken, changedPaths)
				if err != nil {
					logger.Get(ctx).Info("build failed: %v", err.Error())
				}
			}
		}
	}
	return err
}
