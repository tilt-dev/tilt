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
	StartTime  time.Time
	FinishTime time.Time // IsZero() == true for in-progress builds
	Reason     BuildReason

	BuildTypes []BuildType

	// The lookup key for the logs in the logstore.
	SpanID LogSpanID

	// We count the warnings by looking up all the logs with Level=WARNING
	// in the logstore. We store this number separately for ease of use.
	WarningCount int
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
