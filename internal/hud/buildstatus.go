package hud

import (
	"time"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
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

func buildStatusCell(bs buildStatus) rty.Component {
	lhs := rty.NewMinLengthLayout(BuildStatusCellMinWidth, rty.DirHor).
		Add(rty.TextString(bs.status))

	sb := rty.NewStringBuilder()
	if bs.duration != 0 {
		sb.Fg(cLightText).Text(" (")
		sb.Fg(tcell.ColorDefault).Text(formatBuildDuration(bs.duration))
		sb.Fg(cLightText).Text(")")
	}
	rhs := rty.NewMinLengthLayout(BuildDurCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(sb.Build())

	return rty.NewConcatLayout(rty.DirHor).
		Add(lhs).
		Add(rhs)
}
