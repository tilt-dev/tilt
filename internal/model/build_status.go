package model

import (
	"time"
)

const BuildHistoryLimit = 2

type BuildRecord struct {
	Edits      []string
	Error      error
	Warnings   []string
	StartTime  time.Time
	FinishTime time.Time // IsZero() == true for in-progress builds
	Reason     BuildReason
	Log        Log `testdiff:"ignore"`
}

func (bs BuildRecord) Empty() bool {
	return bs.StartTime.IsZero()
}

func (bs BuildRecord) Duration() time.Duration {
	if bs.StartTime.IsZero() {
		return time.Duration(0)
	}
	if bs.FinishTime.IsZero() {
		return time.Since(bs.StartTime)
	}
	return bs.FinishTime.Sub(bs.StartTime)
}
