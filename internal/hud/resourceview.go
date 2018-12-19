package hud

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/rty"
)

// These widths are determined experimentally, to see what shows up in a typical UX.
const DeployCellMinWidth = 8
const BuildDurCellMinWidth = 7
const BuildStatusCellMinWidth = 8

type ResourceView struct {
	res         view.Resource
	rv          view.ResourceViewState
	triggerMode model.TriggerMode
	selected    bool

	clock func() time.Time
}

func NewResourceView(res view.Resource, rv view.ResourceViewState, triggerMode model.TriggerMode,
	selected bool, clock func() time.Time) *ResourceView {
	return &ResourceView{
		res:         res,
		rv:          rv,
		triggerMode: triggerMode,
		selected:    selected,
		clock:       clock,
	}
}

func (v *ResourceView) Build() rty.Component {
	layout := rty.NewConcatLayout(rty.DirVert)
	layout.Add(v.resourceTitle())
	if v.res.IsCollapsed(v.rv) {
		return layout
	}

	layout.Add(v.resourceExpandedPane())
	return layout
}

func (v *ResourceView) resourceTitle() rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(v.titleTextName())
	l.Add(rty.TextString(" "))
	l.AddDynamic(rty.Fg(rty.NewFillerString('╌'), cLightText))
	l.Add(rty.TextString(" "))

	if tt := v.titleText(); tt != nil {
		l.Add(tt)
		l.Add(middotText())
	}

	l.Add(v.titleTextBuild())
	l.Add(middotText())
	l.Add(v.titleTextDeploy())
	return rty.OneLine(l)
}

func (v *ResourceView) statusColor() tcell.Color {
	if dcInfo, ok := v.res.DCInfo(); ok {
		if dcInfo.Status == dockercompose.StatusInProg {
			return cPending
		} else if dcInfo.Status == dockercompose.StatusUp {
			return cGood
		} else if dcInfo.Status == dockercompose.StatusDown {
			return cBad
		}
	} else if !v.res.CurrentBuild.Empty() && !v.res.CurrentBuild.Reason.IsCrashOnly() {
		return cPending
	} else if !v.res.PendingBuildSince.IsZero() && !v.res.PendingBuildReason.IsCrashOnly() {
		if v.triggerMode == model.TriggerAuto {
			return cPending
		} else {
			return cLightText
		}
	} else if isCrashing(v.res) {
		return cBad
	} else if v.res.LastBuild().Error != nil {
		return cBad
	} else if v.res.IsYAMLManifest && !v.res.LastDeployTime.IsZero() {
		return cGood
	} else if !v.res.LastBuild().FinishTime.IsZero() && v.res.PodStatus == "" {
		return cPending // pod status hasn't shown up yet
	}

	statusColor, ok := podStatusColors[v.res.PodStatus]
	if !ok {
		statusColor = cLightText
	}
	return statusColor
}

func (v *ResourceView) titleTextName() rty.Component {
	sb := rty.NewStringBuilder()
	selected := v.selected

	p := " "
	if selected {
		p = "▼"
	}
	if selected && v.res.IsCollapsed(v.rv) {
		p = "▶"
	}

	color := v.statusColor()
	sb.Text(p)
	sb.Fg(color).Textf(" ● ")

	name := v.res.Name.String()
	if color == cPending {
		name = fmt.Sprintf("%s %s", v.res.Name, v.spinner())
	}
	sb.Fg(tcell.ColorDefault).Text(name)
	return sb.Build()
}

func (v *ResourceView) titleTextK8s() rty.Component {
	status := v.res.PodStatus
	if status == "" {
		status = "Pending"
	}
	return rty.TextString(status)
}

func (v *ResourceView) titleText() rty.Component {
	if v.res.IsYAMLManifest {
		return nil
	}
	if tt := v.titleTextDC(); tt != nil {
		return tt
	}
	return v.titleTextK8s()
}

func (v *ResourceView) titleTextDC() rty.Component {
	dcInfo, ok := v.res.DCInfo()
	if !ok {
		return nil
	}

	sb := rty.NewStringBuilder()
	status := dcInfo.Status
	if status == "" {
		status = "Pending"
	}
	sb.Textf("DC %s", status)
	return sb.Build()
}

func (v *ResourceView) titleTextBuild() rty.Component {
	return buildStatusCell(makeBuildStatus(v.res, v.triggerMode))
}

func (v *ResourceView) titleTextDeploy() rty.Component {
	return deployTimeCell(v.res.LastDeployTime, tcell.ColorDefault)
}

func (v *ResourceView) resourceExpandedPane() rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(rty.TextString(strings.Repeat(" ", 4)))

	rhs := rty.NewConcatLayout(rty.DirVert)
	rhs.Add(v.resourceExpandedHistory())
	rhs.Add(v.resourceExpanded())
	rhs.Add(v.resourceExpandedEndpoints())
	rhs.Add(v.resourceExpandedError())
	l.AddDynamic(rhs)
	return l
}

func (v *ResourceView) resourceExpanded() rty.Component {
	if l := v.resourceExpandedDC(); !rty.IsEmpty(l) {
		return l
	}
	if l := v.resourceExpandedK8s(); !rty.IsEmpty(l) {
		return l
	}
	return rty.EmptyLayout
}

func (v *ResourceView) resourceExpandedDC() rty.Component {
	if !v.res.IsDC() {
		return rty.EmptyLayout
	}

	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(v.resourceTextDCContainer())
	l.Add(rty.TextString(" "))
	l.AddDynamic(rty.NewFillerString(' '))

	// TODO(maia): ports

	l.Add(v.resourceTextAge())
	return rty.OneLine(l)
}

func (v *ResourceView) resourceTextDCContainer() rty.Component {
	sb := rty.NewStringBuilder()
	sb.Fg(cLightText).Text("DC container: ")
	sb.Fg(tcell.ColorDefault).Text("not implemented sry 😅")
	return sb.Build()
}

func (v *ResourceView) endpointsNeedSecondLine() bool {
	if len(v.res.Endpoints) > 1 {
		return true
	}
	if len(v.res.Endpoints) == 1 && v.res.PodRestarts > 0 {
		return true
	}
	return false
}

func (v *ResourceView) resourceExpandedK8s() rty.Component {
	if v.res.IsYAMLManifest || v.res.PodName == "" {
		return rty.EmptyLayout
	}

	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(v.resourceTextPodName())
	l.Add(rty.TextString(" "))
	l.AddDynamic(rty.NewFillerString(' '))
	l.Add(rty.TextString(" "))

	if v.res.PodRestarts > 0 {
		l.Add(v.resourceTextPodRestarts())
		l.Add(middotText())
	}

	if len(v.res.Endpoints) > 0 && !v.endpointsNeedSecondLine() {
		for _, endpoint := range v.res.Endpoints {
			l.Add(rty.TextString(endpoint))
			l.Add(middotText())
		}
	}

	l.Add(v.resourceTextAge())
	return rty.OneLine(l)
}

func (v *ResourceView) resourceTextPodName() rty.Component {
	sb := rty.NewStringBuilder()
	sb.Fg(cLightText).Text("K8S POD: ")
	sb.Fg(tcell.ColorDefault).Text(v.res.PodName)
	return sb.Build()
}

func (v *ResourceView) resourceTextPodRestarts() rty.Component {
	s := "restarts"
	if v.res.PodRestarts == 1 {
		s = "restart"
	}
	return rty.NewStringBuilder().
		Fg(cPending).
		Textf("%d %s", v.res.PodRestarts, s).
		Build()
}

func (v *ResourceView) resourceTextAge() rty.Component {
	sb := rty.NewStringBuilder()
	sb.Fg(cLightText).Text("AGE ")
	sb.Fg(tcell.ColorDefault).Text(formatDeployAge(time.Since(v.res.PodCreationTime)))
	return rty.NewMinLengthLayout(DeployCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(sb.Build())
}

func (v *ResourceView) resourceExpandedEndpoints() rty.Component {
	if !v.endpointsNeedSecondLine() {
		return rty.NewConcatLayout(rty.DirVert)
	}

	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(v.resourceTextURL())

	for i, endpoint := range v.res.Endpoints {
		if i != 0 {
			l.Add(middotText())
		}
		l.Add(rty.TextString(endpoint))
	}

	return l
}

func (v *ResourceView) resourceTextURL() rty.Component {
	sb := rty.NewStringBuilder()
	sb.Fg(cLightText).Text("URL: ")
	return sb.Build()
}

func (v *ResourceView) resourceExpandedHistory() rty.Component {
	if v.res.IsYAMLManifest {
		return rty.NewConcatLayout(rty.DirVert)
	}

	if v.res.CurrentBuild.Empty() && len(v.res.BuildHistory) == 0 {
		return rty.NewConcatLayout(rty.DirVert)
	}

	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(rty.NewStringBuilder().Fg(cLightText).Text("HISTORY: ").Build())

	rows := rty.NewConcatLayout(rty.DirVert)
	rowCount := 0
	if !v.res.CurrentBuild.Empty() {
		rows.Add(NewEditStatusLine(buildStatus{
			edits:    v.res.CurrentBuild.Edits,
			reason:   v.res.CurrentBuild.Reason,
			duration: v.res.CurrentBuild.Duration(),
			status:   "Building",
			muted:    true,
		}))
		rowCount++
	}
	for _, bStatus := range v.res.BuildHistory {
		if rowCount >= 2 {
			// at most 2 rows
			break
		}

		status := "OK"
		if bStatus.Error != nil {
			status = "Error"
		}

		rows.Add(NewEditStatusLine(buildStatus{
			edits:      bStatus.Edits,
			reason:     bStatus.Reason,
			duration:   bStatus.Duration(),
			status:     status,
			deployTime: bStatus.FinishTime,
		}))
		rowCount++
	}
	l.AddDynamic(rows)
	return l
}

func (v *ResourceView) resourceExpandedError() rty.Component {
	errPane, ok := v.resourceExpandedBuildError()
	if !ok {
		errPane, ok = v.resourceExpandedK8sError()
	}

	if !ok {
		return rty.NewConcatLayout(rty.DirVert)
	}

	l := rty.NewConcatLayout(rty.DirVert)
	l.Add(rty.NewStringBuilder().Fg(cLightText).Text("ERROR:").Build())

	indentPane := rty.NewConcatLayout(rty.DirHor)
	indentPane.Add(rty.TextString(strings.Repeat(" ", 3)))
	indentPane.AddDynamic(errPane)
	l.Add(indentPane)

	return l
}

// TODO(maia): rename this method to be generic (thiiink it already works with k8s AND dc?)
func (v *ResourceView) resourceExpandedK8sError() (rty.Component, bool) {
	pane := rty.NewConcatLayout(rty.DirVert)
	ok := false
	if isCrashing(v.res) {
		podLog := v.res.CrashLog
		if podLog == "" {
			podLog = v.res.PodLog
		}
		abbrevLog := abbreviateLog(podLog)
		for _, logLine := range abbrevLog {
			pane.Add(rty.TextString(logLine))
			ok = true
		}
	}
	return pane, ok
}

func (v *ResourceView) resourceExpandedBuildError() (rty.Component, bool) {
	pane := rty.NewConcatLayout(rty.DirVert)
	ok := false

	if v.res.LastBuild().Error != nil {
		abbrevLog := abbreviateLog(string(v.res.LastBuild().Log))
		for _, logLine := range abbrevLog {
			pane.Add(rty.TextString(logLine))
			ok = true
		}

		// if the build log is non-empty, it will contain the error, so we don't need to show this separately
		if len(abbrevLog) == 0 {
			pane.Add(rty.TextString(fmt.Sprintf("Error: %s", v.res.LastBuild().Error)))
			ok = true
		}
	}

	return pane, ok
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (v *ResourceView) spinner() string {
	decisecond := v.clock().Nanosecond() / int(time.Second/10)
	return spinnerChars[decisecond%len(spinnerChars)] // tick spinner every 10x/second
}
