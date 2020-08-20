package fswatch

import (
	"time"

	"github.com/tilt-dev/tilt/pkg/model"
)

type TargetFilesChangedAction struct {
	TargetID model.TargetID
	Files    []string
	Time     time.Time
}

func (TargetFilesChangedAction) Action() {}

func NewTargetFilesChangedAction(targetID model.TargetID, files ...string) TargetFilesChangedAction {
	return TargetFilesChangedAction{
		TargetID: targetID,
		Files:    files,
		Time:     time.Now(),
	}
}

type GitBranchStatusAction struct {
	Time time.Time
	Repo model.LocalGitRepo
	Head string
}

func (GitBranchStatusAction) Action() {}
