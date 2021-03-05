package fswatch

import (
	"time"

	"k8s.io/apimachinery/pkg/types"

	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type FileWatchCreateAction struct {
	FileWatch *filewatches.FileWatch
}

func (FileWatchCreateAction) Action() {}

func NewFileWatchCreateAction(fw *filewatches.FileWatch) FileWatchCreateAction {
	return FileWatchCreateAction{FileWatch: fw.DeepCopy()}
}

type FileWatchUpdateAction struct {
	FileWatch *filewatches.FileWatch
}

func (FileWatchUpdateAction) Action() {}

func NewFileWatchUpdateAction(fw *filewatches.FileWatch) FileWatchUpdateAction {
	return FileWatchUpdateAction{FileWatch: fw.DeepCopy()}
}

type FileWatchUpdateStatusAction struct {
	Name   types.NamespacedName
	Status *filewatches.FileWatchStatus
}

func (FileWatchUpdateStatusAction) Action() {}

func NewFileWatchUpdateStatusAction(name types.NamespacedName, fwStatus *filewatches.FileWatchStatus) FileWatchUpdateStatusAction {
	return FileWatchUpdateStatusAction{Name: name, Status: fwStatus.DeepCopy()}
}

type FileWatchDeleteAction struct {
	Name types.NamespacedName
}

func (FileWatchDeleteAction) Action() {}

func NewFileWatchDeleteAction(name types.NamespacedName) FileWatchDeleteAction {
	return FileWatchDeleteAction{Name: name}
}

type GitBranchStatusAction struct {
	Time time.Time
	Repo model.LocalGitRepo
	Head string
}

func (GitBranchStatusAction) Action() {}
