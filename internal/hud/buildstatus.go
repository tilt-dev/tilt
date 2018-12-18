package hud

import (
	"time"

	"github.com/gdamore/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/rty"
)

type buildStatus struct {
	edits      []string
	duration   time.Duration
	status     string
	deployTime time.Time
}

func makeBuildStatus(res view.Resource, triggerMode model.TriggerMode) buildStatus {
	status := "Pending"
	duration := time.Duration(0)
	edits := []string{}
	deployTime := time.Time{}

	if !res.CurrentBuild.StartTime.IsZero() && !res.CurrentBuild.Reason.IsCrashOnly() {
		status = "Building"
		duration = time.Since(res.CurrentBuild.StartTime)
		edits = res.CurrentBuild.Edits
	} else if !res.PendingBuildSince.IsZero() && !res.PendingBuildReason.IsCrashOnly() {
		status = "Pending"
		if triggerMode == model.TriggerAuto {
			duration = time.Since(res.PendingBuildSince)
		}
		edits = res.PendingBuildEdits
	} else if !res.LastBuild().FinishTime.IsZero() {
		if res.LastBuild().Error != nil {
			status = "Error"
		} else {
			status = "OK"
		}
		duration = res.LastBuild().Duration()
		edits = res.LastBuild().Edits
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
