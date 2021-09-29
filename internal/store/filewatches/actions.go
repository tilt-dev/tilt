package filewatches

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type FileWatchUpsertAction struct {
	FileWatch *v1alpha1.FileWatch
}

func NewFileWatchUpsertAction(obj *v1alpha1.FileWatch) FileWatchUpsertAction {
	return FileWatchUpsertAction{FileWatch: obj}
}

func (FileWatchUpsertAction) Action() {}

type FileWatchDeleteAction struct {
	Name string
}

func NewFileWatchDeleteAction(n string) FileWatchDeleteAction {
	return FileWatchDeleteAction{Name: n}
}

func (FileWatchDeleteAction) Action() {}
