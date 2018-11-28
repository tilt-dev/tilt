package hud

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/rty"
)

type Renderer struct {
	rty    rty.RTY
	screen tcell.Screen
	mu     *sync.Mutex
	clock  func() time.Time
}

func NewRenderer(clock func() time.Time) *Renderer {
	return &Renderer{
		mu:    new(sync.Mutex),
		clock: clock,
	}
}

func (r *Renderer) Render(v view.View, vs view.ViewState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rty != nil {
		layout := r.layout(v, vs)
		err := r.rty.Render(layout)
		if err != nil {
			return err
		}
	}
	return nil
}

func formatPreciseDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	seconds := int(d.Seconds())
	if seconds > 10 {
		return fmt.Sprintf("%ds", seconds)
	}

	fractionalSeconds := float64(d) / float64(time.Second)
	return fmt.Sprintf("%0.2fs", fractionalSeconds)
}

func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}

	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}

	return "<1m"
}

func formatFileList(files []string) string {
	const maxFilesToDisplay = 3

	var ret []string

	for i, f := range files {
		if i > maxFilesToDisplay {
			ret = append(ret, fmt.Sprintf("(%d more)", len(files)-maxFilesToDisplay))
			break
		}
		ret = append(ret, f)
	}

	return strings.Join(ret, ", ")
}

var cText = tcell.Color232
var cLightText = tcell.Color243
var cGood = tcell.ColorGreen
var cBad = tcell.ColorRed
var cPending = tcell.ColorYellow

var podStatusColors = map[string]tcell.Color{
	"Running":           cGood,
	"ContainerCreating": cPending,
	"Pending":           cPending,
	"Error":             cBad,
	"CrashLoopBackOff":  cBad,
}

func (r *Renderer) layout(v view.View, vs view.ViewState) rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)
	if vs.ShowNarration {
		l.Add(renderNarration(vs.NarrationMessage))
		l.Add(rty.NewLine())
	}

	split := rty.NewFlexLayout(rty.DirVert)

	split.Add(r.renderResources(v, vs))
	split.Add(r.renderFooter(v, keyLegend(vs)))
	l.Add(split)

	var ret rty.Component = l
	if vs.LogModal.TiltLog {
		ret = r.renderFullLogModal(v, ret)
	} else if vs.LogModal.ResourceLogNumber != 0 {
		ret = r.renderResourceLogModal(v.Resources[vs.LogModal.ResourceLogNumber-1], ret)
	}

	ret = r.maybeAddAlertModal(vs, ret)

	return ret
}

func (r *Renderer) maybeAddAlertModal(vs view.ViewState, layout rty.Component) rty.Component {
	if vs.AlertMessage != "" {
		b := rty.NewBox(rty.Fg(rty.TextString(vs.AlertMessage), tcell.ColorDefault))
		b.SetTitle("! Alert !")
		layout = r.renderModal(rty.Fg(b, tcell.ColorRed), layout, false)
	}
	return layout
}

func keyLegend(vs view.ViewState) string {
	defaultKeys := "(↓) next, (↑) prev ┊ (→) expand, (←) collapse, (enter) log, (b)rowser ┊ Tilt (l)og ┊ (q)uit  "
	if vs.LogModal.TiltLog || vs.LogModal.ResourceLogNumber != 0 {
		return "SCROLL: (↓) (↑) ┊ (esc) to exit view "
	}
	return defaultKeys
}

func (r *Renderer) renderFooter(v view.View, keys string) rty.Component {
	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()

	sbLeft.Text(" ") // Indent
	errorCount := 0
	for _, res := range v.Resources {
		if isInError(res) {
			errorCount++
		}
	}
	if errorCount == 0 {
		sbLeft.Fg(cGood).Text("✓").Fg(tcell.ColorDefault).Fg(cText).Text(" OK").Fg(tcell.ColorDefault)
	} else {
		s := "error"
		if errorCount > 1 {
			s = "errors"
		}
		sbLeft.Fg(cBad).Text("✖").Fg(tcell.ColorDefault).Fg(cText).Textf(" %d %s", errorCount, s).Fg(tcell.ColorDefault)
	}
	sbRight.Fg(cText).Text(keys).Fg(tcell.ColorDefault)

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())

	return rty.NewFixedSize(rty.Bg(l, tcell.ColorWhiteSmoke), rty.GROW, 1)
}

func isInError(res view.Resource) bool {
	return res.LastBuildError != "" || podStatusColors[res.PodStatus] == cBad
}

func (r *Renderer) renderFullLogModal(v view.View, background rty.Component) rty.Component {
	return r.renderLogModal("TILT LOG", v.Log, background)
}

func (r *Renderer) renderResourceLogModal(res view.Resource, background rty.Component) rty.Component {
	var s string
	if res.LastBuildError != "" && len(strings.TrimSpace(res.LastBuildLog)) > 0 {
		s = res.LastBuildLog
	} else if len(strings.TrimSpace(res.PodLog)) > 0 {
		s = res.PodLog
	} else {
		s = fmt.Sprintf("No log output for %s", res.Name)
	}

	return r.renderLogModal(fmt.Sprintf("POD LOG: %s", res.Name), s, background)
}

func (r *Renderer) renderLogModal(title string, s string, background rty.Component) rty.Component {
	sl := rty.NewTextScrollLayout(logScrollerName)
	sl.Add(rty.TextString(s))
	box := rty.NewGrowingBox()
	box.SetInner(sl)
	box.SetTitle(title)

	return r.renderModal(box, background, true)
}

func (r *Renderer) renderModal(fg rty.Component, bg rty.Component, fixed bool) rty.Component {
	return rty.NewModalLayout(bg, fg, .9, fixed)
}

func renderNarration(msg string) rty.Component {
	lines := rty.NewLines()
	l := rty.NewLine()
	l.Add(rty.TextString(msg))
	lines.Add(rty.NewLine())
	lines.Add(l)
	lines.Add(rty.NewLine())

	box := rty.Fg(rty.Bg(lines, tcell.ColorLightGrey), cText)
	return rty.NewFixedSize(box, rty.GROW, 3)
}

func (r *Renderer) renderResources(v view.View, vs view.ViewState) rty.Component {
	rs := v.Resources
	childNames := make([]string, len(rs))
	for i, r := range rs {
		childNames[i] = r.Name
	}

	l, selectedResource := r.rty.RegisterElementScroll(resourcesScollerName, childNames)

	if len(rs) > 0 {
		for i, res := range rs {
			l.Add(r.renderResource(res, vs.Resources[i], selectedResource == res.Name))
		}
	}

	return l
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (r *Renderer) spinner() string {
	return spinnerChars[r.clock().Second()%len(spinnerChars)]
}

const abbreviatedLogLineCount = 6

func abbreviateLog(s string) []string {
	lines := strings.Split(s, "\n")
	start := len(lines) - abbreviatedLogLineCount
	if start < 0 {
		start = 0
	}

	// skip past leading empty lines
	for {
		if start < len(lines) && len(strings.TrimSpace(lines[start])) == 0 {
			start++
		} else {
			break
		}
	}

	return lines[start:]
}

func (r *Renderer) renderResource(res view.Resource, rv view.ResourceViewState, selected bool) rty.Component {
	layout := rty.NewConcatLayout(rty.DirVert)
	layout.Add(r.resourceTitle(selected, rv, res))
	if l := r.resourceK8s(res, rv); l != nil {
		layout.Add(l)
	}
	layout.Add(r.resourceK8sLogs(res, rv))
	layout.Add(r.resourceTilt(res, rv))
	return layout
}

func (r *Renderer) resourceTitle(selected bool, rv view.ResourceViewState, res view.Resource) rty.Component {
	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()

	p := " "
	if selected {
		p = "▼"
	}
	if selected && rv.IsCollapsed {
		p = "▶"
	}

	sbLeft.Textf("%s %s ", p, res.Name)

	if res.LastDeployTime.Equal(time.Time{}) {
		sbRight.Text(" Not Deployed •  —      ")
	} else {
		sbRight.Textf(" OK • %s ago ", formatDuration(time.Since(res.LastDeployTime))) // Last char cuts off
	}

	l.Add(sbLeft.Build())
	l.Add(rty.Fg(rty.NewFillerString('╌'), cLightText))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceK8s(res view.Resource, rv view.ResourceViewState) rty.Component {
	if res.IsYAMLManifest {
		return nil
	}

	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()
	status := r.spinner()
	if !res.LastBuildFinishTime.Equal(time.Time{}) && res.LastDeployTime.Equal(time.Time{}) {
		// We have a finished build but aren't deployed, because the build is broken
		status = "N/A"
	}
	indent := strings.Repeat(" ", 8)

	if res.PodStatus != "" {
		podStatusColor, ok := podStatusColors[res.PodStatus]
		if !ok {
			podStatusColor = tcell.ColorDefault
		}
		sbLeft.Fg(podStatusColor).Textf("%s●  ", indent).Fg(tcell.ColorDefault)
		status = res.PodStatus

		// TODO(maia): show # restarts even if == 0 (in gray or green)?
		if res.PodRestarts > 0 {
			s := "restarts"
			if res.PodRestarts == 1 {
				s = "restart"
			}
			sbRight.Fg(cPending).Textf("%d %s", res.PodRestarts, s).Fg(tcell.ColorDefault).Text(" • ")
		}

		if len(res.Endpoints) != 0 {
			sbRight.Textf("%s • ", strings.Join(res.Endpoints, " "))
		}

		sbRight.Fg(cLightText).Text("AGE").Fg(tcell.ColorDefault)
		sbRight.Textf(" %s ", formatDuration(time.Since(res.PodCreationTime))) // Last char cuts off
	} else {
		sbLeft.Fg(cLightText).Textf("%s●  ", indent).Fg(tcell.ColorDefault)
	}

	sbLeft.Fg(cLightText).Textf("K8S: ").Fg(tcell.ColorDefault).Text(status)

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceTilt(res view.Resource, rv view.ResourceViewState) rty.Component {
	lines := rty.NewLines()
	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()

	indent := strings.Repeat(" ", 8)
	statusColor := cPending
	status := r.spinner()
	duration := formatPreciseDuration(res.LastBuildDuration)
	editsPrefix := ""
	edits := ""

	if res.LastManifestLoadError != "" {
		statusColor = cBad
		status = "Problem loading Tiltfile"
	} else if !res.LastBuildFinishTime.Equal(time.Time{}) {
		if res.LastBuildError != "" {
			statusColor = cBad
			status = "ERROR"
		} else {
			statusColor = cGood
			status = "OK"
		}
	}

	sbLeft.Fg(statusColor).Textf("%s●", indent).Fg(tcell.ColorDefault)
	sbLeft.Fg(cLightText).Text(" TILT: ").Fg(tcell.ColorDefault).Text(status)

	if !res.LastDeployTime.Equal(time.Time{}) {
		if len(res.LastDeployEdits) > 0 {
			editsPrefix = " • EDITS "
			edits = formatFileList(res.LastDeployEdits)
		}
	}
	if !res.CurrentBuildStartTime.Equal(time.Time{}) {
		if len(res.CurrentBuildEdits) > 0 {
			editsPrefix = " • EDITS "
			edits = formatFileList(res.CurrentBuildEdits)
			status = "In Progress"
		}
		duration = formatPreciseDuration(time.Since(res.CurrentBuildStartTime))
	}
	if !res.PendingBuildSince.Equal(time.Time{}) {
		if len(res.PendingBuildEdits) > 0 {
			editsPrefix = " • EDITS "
			edits = formatFileList(res.PendingBuildEdits)
		}
		duration = formatPreciseDuration(time.Since(res.PendingBuildSince))
	}

	sbLeft.Fg(cLightText).Text(editsPrefix).Fg(tcell.ColorDefault).Text(edits)
	sbRight.Fg(cLightText).Text("DURATION ")
	sbRight.Fg(tcell.ColorDefault).Textf("%s           ", duration) // Last char cuts off

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	lines.Add(l)
	lines.Add(r.lastBuildLogs(res, rv))
	return lines
}

func (r *Renderer) resourceK8sLogs(res view.Resource, rv view.ResourceViewState) rty.Component {
	lh := rty.NewConcatLayout(rty.DirHor)
	lv := rty.NewConcatLayout(rty.DirVert)
	lh.AddDynamic(lv)
	var logLines []rty.Component
	indent := strings.Repeat(" ", 12)

	if res.PodStatus != "" && !rv.IsCollapsed {
		if res.PodRestarts > 0 {
			abbrevLog := abbreviateLog(res.PodLog)
			for _, logLine := range abbrevLog {
				logLines = append(logLines, rty.TextString(fmt.Sprintf("%s%s", indent, logLine)))
			}
			if len(logLines) > 0 {
				for _, log := range logLines {
					lv.Add(log)
				}
			}
		}
	}

	return lh
}

func (r *Renderer) lastBuildLogs(res view.Resource, rv view.ResourceViewState) rty.Component {
	lh := rty.NewConcatLayout(rty.DirHor)
	lv := rty.NewConcatLayout(rty.DirVert)
	lh.AddDynamic(lv)
	var logLines []rty.Component
	indent := strings.Repeat(" ", 12)

	if res.LastManifestLoadError != "" {
		logLines = append(logLines, rty.TextString(fmt.Sprintf("%s%s", indent, res.LastManifestLoadError)))
	}

	if !rv.IsCollapsed {
		if res.LastBuildError != "" {
			abbrevLog := abbreviateLog(res.LastBuildLog)
			for _, logLine := range abbrevLog {
				logLines = append(logLines, rty.TextString(fmt.Sprintf("%s%s", indent, logLine)))
			}
			// if the build log is non-empty, it will contain the error, so we don't need to show this separately
			if len(abbrevLog) == 0 {
				logLines = append(logLines, rty.TextString(fmt.Sprintf("%sError: %s", indent, res.LastBuildError)))
			}
		}

		for _, log := range logLines {
			lv.Add(log)
		}
	}

	return lh
}

func (r *Renderer) SetUp() (chan tcell.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	if err = screen.Init(); err != nil {
		return nil, err
	}
	screenEvents := make(chan tcell.Event)
	go func() {
		for {
			screenEvents <- screen.PollEvent()
		}
	}()

	r.rty = rty.NewRTY(screen)

	r.screen = screen

	return screenEvents, nil
}

func (r *Renderer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.screen != nil {
		r.screen.Fini()
	}

	r.screen = nil
}
