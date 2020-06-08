package fswatch

import (
	"time"

	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type FsWatcherMaker func(paths []string, ignore watch.PathMatcher, l logger.Logger) (watch.Notify, error)

type TimerMaker func(d time.Duration) <-chan time.Time

func ProvideFsWatcherMaker() FsWatcherMaker {
	return func(paths []string, ignore watch.PathMatcher, l logger.Logger) (watch.Notify, error) {
		return watch.NewWatcher(paths, ignore, l)
	}
}

func ProvideTimerMaker() TimerMaker {
	return func(t time.Duration) <-chan time.Time {
		return time.After(t)
	}
}
