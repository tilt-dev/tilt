package watch

import "github.com/windmilleng/tilt/internal/model"

type FileEvent struct {
	Path string
}

type Notify interface {
	Close() error
	Add(name string, filter model.PathMatcher) error
	Events() chan FileEvent
	Errors() chan error
}
