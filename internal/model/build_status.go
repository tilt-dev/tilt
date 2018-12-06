package model

import (
	"time"
)

const BuildHistoryLimit = 2

type BuildStatus struct {
	Edits      []string
	Error      error
	StartTime  time.Time
	FinishTime time.Time // IsZero() == true for in-progress builds
	Reason     BuildReason
	Log        []byte `testdiff:"ignore"`
}

func (bs BuildStatus) Empty() bool {
	return bs.StartTime.IsZero()
}

func (bs BuildStatus) Duration() time.Duration {
	if bs.StartTime.IsZero() {
		return time.Duration(0)
	}
	if bs.FinishTime.IsZero() {
		return time.Since(bs.StartTime)
	}
	return bs.FinishTime.Sub(bs.StartTime)
}
