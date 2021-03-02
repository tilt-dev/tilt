package filewatch

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/tilt-dev/fsnotify"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/watch"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/clientset/tiltapi"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const maxFileWatchHistory = 20

// NotifyClient is the minimal set of methods for FileWatch API that the manager needs.
//
// This matches the methods from tiltapi FileWatchInterface, NOT the controller client!
// The controller client is only suitable for usage _within_ reconciliation as it does
// its own caching.
type NotifyClient interface {
	Get(ctx context.Context, name string, opts v1.GetOptions) (*filewatches.FileWatch, error)
	UpdateStatus(ctx context.Context, fileWatch *filewatches.FileWatch, opts v1.UpdateOptions) (*filewatches.FileWatch, error)
}

func ProvideNotifyClient(tiltApiClient tiltapi.Interface) NotifyClient {
	return tiltApiClient.CoreV1alpha1().FileWatches()
}

type ApiServerWatchManager struct {
	client NotifyClient

	watcherMaker watch.FsWatcherMaker
	timerMaker   watch.TimerMaker

	mu      sync.Mutex
	watches map[string]*fsWatch
}

type fsWatch struct {
	name string
	spec     filewatches.FileWatchSpec
	logger logger.Logger
	notifier watch.Notify
	cancel   func()
}

func NewApiServerWatchManager(notifyClient NotifyClient, watcherMaker watch.FsWatcherMaker, timerMaker watch.TimerMaker) *ApiServerWatchManager {
	return &ApiServerWatchManager{
		client:       notifyClient,
		watcherMaker: watcherMaker,
		timerMaker:   timerMaker,
		watches:      make(map[string]*fsWatch),
	}
}

func (m *ApiServerWatchManager) StopWatch(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.watches[name]
	if !ok {
		return
	}

	m.cleanupAndRemoveWatch(w)
}

// StartWatch adds a new filesystem watch for a FileWatch API object.
//
// If a watch for the spec already exists and the spec has not changed, the existing watch is left untouched
// and the boolean return value is false. If there is no watch or the spec has changed, it is replaced and
// the boolean return value is true.
func (m *ApiServerWatchManager) StartWatch(ctx context.Context, name string, spec filewatches.FileWatchSpec) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldWatch := m.watches[name]
	if oldWatch != nil {
		if equality.Semantic.DeepEqual(oldWatch.spec, spec) {
			return false, nil
		}
	}

	ignoreMatcher, err := dockerignore.NewDockerPatternMatcher(spec.RootPath, spec.IgnorePatterns)
	if err != nil {
		return false, err
	}

	watchPaths, err := absPaths(spec.RootPath, spec.Paths)
	if err != nil {
		return false, err
	}

	notifier, err := m.watcherMaker(watchPaths, ignoreMatcher, logger.Get(ctx))
	if err != nil {
		return false, err
	}

	if err := notifier.Start(); err != nil {
		return false, err
	}

	watchCtx, cancel := context.WithCancel(ctx)
	w := &fsWatch{
		name: name,
		spec:     spec,
		logger: logger.Get(watchCtx),
		notifier: notifier,
		cancel:   cancel,
	}
	go m.notifyLoop(watchCtx, w)

	// the old watch is cleaned up AFTER starting the new one to avoid missing events
	// (this might mean duplicates are received, which is an acceptable trade-off)
	if oldWatch != nil {
		m.cleanupAndRemoveWatch(oldWatch)
	}

	m.watches[name] = w

	return true, nil
}

func (m *ApiServerWatchManager) cleanupAndRemoveWatch(w *fsWatch) {
	if err := w.notifier.Close(); err != nil {
		w.logger.Debugf("Error cleaning up FS watch for %q: %v", w)
	}
	w.cancel()

	entry := m.watches[w.name]
	if entry == w {
		delete(m.watches, w.name)
	}
}

type updateStatusFunc func(status *filewatches.FileWatchStatus)

func (m *ApiServerWatchManager) updateStatus(ctx context.Context, name string, updateFunc updateStatusFunc) error {
	obj, err := m.client.Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return err
	}
	updateFunc(&obj.Status)
	_, err = m.client.UpdateStatus(ctx, obj, v1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (m *ApiServerWatchManager) notifyLoop(ctx context.Context, w *fsWatch) {
	eventsCh := watch.CoalesceEvents(m.timerMaker, w.notifier.Events())

	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.cleanupAndRemoveWatch(w)
	}()

	for {
		select {
		case fsEvents, ok := <-eventsCh:
			if !ok {
				return
			}

			err := m.updateStatus(ctx, w.name, func(status *filewatches.FileWatchStatus) {
				now := v1.Now()
				status.LastEventTime = &now
				status.ErrorMessage = ""
				status.SeenFiles = seenFiles(fsEvents, status.SeenFiles)
			})
			if err != nil {
				w.logger.Debugf("Failed to record FS events for %q: %v", w.name, err)
			}
		case err, ok := <-w.notifier.Errors():
			if !ok {
				return
			}

			var errorMessage string
			if watch.IsWindowsShortReadError(err) {
				errorMessage = watch.WindowsShortReadErrorMessage(err)
			} else if err.Error() == fsnotify.ErrEventOverflow.Error() {
				errorMessage = watch.DetectedOverflowErrorMessage(err)
			} else {
				errorMessage = err.Error()
			}

			updateErr := m.updateStatus(ctx, w.name, func(status *filewatches.FileWatchStatus) {
				now := v1.Now()
				status.LastEventTime = &now
				status.ErrorMessage = errorMessage
			})
			if updateErr != nil {
				// TODO(milas): make this log message coherent (also - should this be fatal?)
				w.logger.Debugf("Failed to update FileWatch: %v", updateErr)
			}
		case <-ctx.Done():
			return
		}
	}
}

func seenFiles(events []watch.FileEvent, current []string) []string {
	size := len(events) + len(current)
	if size > maxFileWatchHistory {
		size = maxFileWatchHistory
	}

	eventPaths := make(map[string]bool, len(events))
	seen := make([]string, 0, size)
	for _, e := range events {
		if len(seen) >= maxFileWatchHistory {
			break
		}
		p := e.Path()
		if !eventPaths[p] {
			eventPaths[p] = true
			seen = append(seen, p)
		}
	}

	for _, path := range current {
		if len(seen) >= maxFileWatchHistory {
			break
		}
		if !eventPaths[path] {
			seen = append(seen, path)
		}
	}

	return seen
}

// absPaths returns absolute paths for all paths or an error if a path that is already absolute is encountered.
func absPaths(rootDir string, paths []string) ([]string, error) {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			return nil, fmt.Errorf("path is not relative: %q", path)
		}
		out = append(out, filepath.Join(rootDir, path))
	}
	return out, nil
}
