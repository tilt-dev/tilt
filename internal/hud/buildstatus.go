package hud

import (
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
)

type buildStatus struct {
	edits      []string
	duration   time.Duration
	status     string
	deployTime time.Time
}

func makeBuildStatus(res view.Resource) buildStatus {
	status := "Pending …"
	duration := time.Duration(0)
	edits := []string{}
	deployTime := time.Time{}

	if !res.CurrentBuildStartTime.IsZero() && !res.CurrentBuildReason.IsCrashOnly() {
		status = "Building"
		duration = time.Since(res.CurrentBuildStartTime)
		edits = res.CurrentBuildEdits
	} else if !res.PendingBuildSince.IsZero() && !res.PendingBuildReason.IsCrashOnly() {
		status = "Pending …"
		duration = time.Since(res.PendingBuildSince)
		edits = res.PendingBuildEdits
	} else if !res.LastBuildFinishTime.IsZero() {
		if res.LastBuildError != "" {
			status = "Build Error"
		} else {
			status = "Build OK"
		}
		duration = res.LastBuildDuration
		edits = res.LastBuildEdits
		deployTime = res.LastDeployTime

	}

	return buildStatus{
		status:     status,
		duration:   duration,
		edits:      edits,
		deployTime: deployTime,
	}
}
