package model

import (
	"time"
)

const BuildHistoryLimit = 2

type BuildType string

const BuildTypeImage BuildType = "image"
const BuildTypeLiveUpdate BuildType = "live-update"
const BuildTypeDockerCompose BuildType = "docker-compose"
const BuildTypeK8s BuildType = "k8s"
const BuildTypeLocal BuildType = "local"

type BuildRecord struct {
	Edits      []string
	Error      error
	Warnings   []string
	StartTime  time.Time
	FinishTime time.Time // IsZero() == true for in-progress builds
	Reason     BuildReason

	// TODO(nick): Delete Log and use SpanID to load
	// the log from the logstore.
	Log Log `testdiff:"ignore"`

	BuildTypes []BuildType

	// The lookup key for the logs in the logstore.
	SpanID LogSpanID
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

func (r BuildRecord) HasBuildType(bt BuildType) bool {
	for _, el := range r.BuildTypes {
		if el == bt {
			return true
		}
	}
	return false
}
