package watch

import (
	"path/filepath"
	"time"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/fsnotify/fsevents"
)

// A file watcher optimized for Darwin.
// Uses FSEvents to avoid the terrible perf characteristics of kqueue.
type darwinNotify struct {
	stream *fsevents.EventStream
	events chan FileEvent
	errors chan error
	stop   chan struct{}

	pathsWereWatching map[string]interface{}
	ignore            PathMatcher
	logger            logger.Logger
}

func (d *darwinNotify) loop() {
	for {
		select {
		case <-d.stop:
			return
		case events, ok := <-d.stream.Events:
			if !ok {
				return
			}

			for _, e := range events {
				e.Path = filepath.Join("/", e.Path)

				_, isPathWereWatching := d.pathsWereWatching[e.Path]
				isDir := e.Flags&fsevents.ItemIsDir == fsevents.ItemIsDir
				if isDir && isPathWereWatching {
					// For consistency with Linux and Windows, don't fire any events
					// for directories that we're watching -- only their contents.
					continue
				}

				// On MacOS, modifying a directory entry fires Created | InodeMetaMod
				// Ignore these events, mod time modifications shouldnt trigger copies.
				if isDir && (e.Flags&fsevents.ItemInodeMetaMod) == fsevents.ItemInodeMetaMod {
					continue
				}

				ignore, err := d.ignore.Matches(e.Path)
				if err != nil {
					d.logger.Infof("Error matching path %q: %v", e.Path, err)
				} else if ignore {
					continue
				}

				d.events <- NewFileEvent(e.Path)
			}
		}
	}
}

// Add a path to be watched. Should only be called during initialization.
func (d *darwinNotify) initAdd(name string) {
	d.stream.Paths = append(d.stream.Paths, name)

	if d.pathsWereWatching == nil {
		d.pathsWereWatching = make(map[string]interface{})
	}
	d.pathsWereWatching[name] = struct{}{}
}

func (d *darwinNotify) Start() error {
	if len(d.stream.Paths) == 0 {
		return nil
	}

	numberOfWatches.Add(int64(len(d.stream.Paths)))

	if err := d.stream.Start(); err != nil {
		return err
	}

	go d.loop()

	return nil
}

func (d *darwinNotify) Close() error {
	numberOfWatches.Add(int64(-len(d.stream.Paths)))

	d.stream.Stop()
	close(d.errors)
	close(d.stop)

	return nil
}

func (d *darwinNotify) Events() chan FileEvent {
	return d.events
}

func (d *darwinNotify) Errors() chan error {
	return d.errors
}

func newWatcher(paths []string, ignore PathMatcher, l logger.Logger) (*darwinNotify, error) {
	dw := &darwinNotify{
		ignore: ignore,
		logger: l,
		stream: &fsevents.EventStream{
			Latency: 1 * time.Millisecond,
			Flags:   fsevents.FileEvents,
			// NOTE(dmiller): this corresponds to the `sinceWhen` parameter in FSEventStreamCreate
			// https://developer.apple.com/documentation/coreservices/1443980-fseventstreamcreate
			EventID: fsevents.LatestEventID(),
		},
		events: make(chan FileEvent),
		errors: make(chan error),
		stop:   make(chan struct{}),
	}

	paths = dedupePathsForRecursiveWatcher(paths)
	for _, path := range paths {
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, errors.Wrap(err, "newWatcher")
		}
		dw.initAdd(path)
	}

	return dw, nil
}

var _ Notify = &darwinNotify{}
