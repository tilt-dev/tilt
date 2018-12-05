package store

import (
	"time"

	"github.com/windmilleng/tilt/internal/model"
)

const BuildHistoryLimit = 2

type BuildStatus struct {
	Edits      []string
	Error      error
	StartTime  time.Time
	FinishTime time.Time // IsZero() == true for in-progress builds
	Reason     model.BuildReason
	Log        []byte `testdiff:"ignore"`
}

func (bs BuildStatus) Empty() bool {
	return bs.StartTime.IsZero()
}

func (bs BuildStatus) Duration() time.Duration {
	if bs.FinishTime.IsZero() {
		return time.Since(bs.StartTime)
	}
	return bs.FinishTime.Sub(bs.StartTime)
}
