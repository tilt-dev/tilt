package hud

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

// These widths are determined experimentally, to see what shows up in a typical UX.
const DeployCellMinWidth = 8
const BuildDurCellMinWidth = 7
const BuildStatusCellMinWidth = 8

type ResourceView struct {
	res      view.Resource
	rv       view.ResourceViewState
	selected bool

	clock func() time.Time
}

func NewResourceView(res view.Resource, rv view.ResourceViewState, selected bool, clock func() time.Time) *ResourceView {
	return &ResourceView{
		res:      res,
		rv:       rv,
		selected: selected,
		clock:    clock,
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

	if !v.res.IsYAMLManifest {
		l.Add(v.titleTextK8s())
		l.Add(middotText())
	}

	l.Add(v.titleTextBuild())
	l.Add(middotText())
	l.Add(v.titleTextDeploy())
	return rty.OneLine(l)
}

func (v *ResourceView) statusColor() tcell.Color {
	if !v.res.CurrentBuildStartTime.IsZero() && !v.res.CurrentBuildReason.IsCrashOnly() {
		return cPending
	} else if !v.res.PendingBuildSince.IsZero() && !v.res.PendingBuildReason.IsCrashOnly() {
		return cPending
	} else if isCrashing(v.res) {
		return cBad
	} else if v.res.LastBuildError != "" {
		return cBad
	} else if v.res.IsYAMLManifest && !v.res.LastDeployTime.IsZero() {
		return cGood
	} else if !v.res.LastBuildFinishTime.IsZero() && v.res.PodStatus == "" {
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

	name := v.res.Name
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

func (v *ResourceView) titleTextBuild() rty.Component {
	return buildStatusCell(makeBuildStatus(v.res))
}

func (v *ResourceView) titleTextDeploy() rty.Component {
	return deployTimeCell(v.res.LastDeployTime)
}

func (v *ResourceView) resourceExpandedPane() rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(rty.TextString(strings.Repeat(" ", 4)))

	rhs := rty.NewConcatLayout(rty.DirVert)
	rhs.Add(v.resourceExpandedK8s())
	rhs.Add(v.resourceExpandedEndpoints())
	rhs.Add(v.resourceExpandedHistory())
	rhs.Add(v.resourceExpandedError())
	l.AddDynamic(rhs)
	return l
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
		return rty.NewConcatLayout(rty.DirVert)
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

	if len(v.res.CurrentBuildEdits) == 0 && len(v.res.LastBuildEdits) == 0 {
		return rty.NewConcatLayout(rty.DirVert)
	}

	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(rty.NewStringBuilder().Fg(cLightText).Text("HISTORY: ").Build())

	rows := rty.NewConcatLayout(rty.DirVert)
	if len(v.res.CurrentBuildEdits) != 0 {
		rows.Add(NewEditStatusLine(buildStatus{
			edits:    v.res.CurrentBuildEdits,
			duration: time.Since(v.res.CurrentBuildStartTime),
			status:   "Building",
		}))
	}
	if len(v.res.LastBuildEdits) != 0 {
		status := "OK"
		if v.res.LastBuildError != "" {
			status = "Error"
		}
		rows.Add(NewEditStatusLine(buildStatus{
			edits:      v.res.LastBuildEdits,
			duration:   v.res.LastBuildDuration,
			status:     status,
			deployTime: v.res.LastDeployTime,
		}))
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

	if v.res.LastBuildError != "" {
		abbrevLog := abbreviateLog(v.res.LastBuildLog)
		for _, logLine := range abbrevLog {
			pane.Add(rty.TextString(logLine))
			ok = true
		}

		// if the build log is non-empty, it will contain the error, so we don't need to show this separately
		if len(abbrevLog) == 0 {
			pane.Add(rty.TextString(fmt.Sprintf("Error: %s", v.res.LastBuildError)))
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
