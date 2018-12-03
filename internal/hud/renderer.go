package hud

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
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
		l := rty.NewLines()
		l.Add(rty.TextString(""))

		msg := "   " + vs.AlertMessage + "   "
		l.Add(rty.Fg(rty.TextString(msg), tcell.ColorDefault))
		l.Add(rty.TextString(""))

		b := rty.NewBox(l)
		b.SetTitle("! Alert !")
		layout = r.renderModal(rty.Fg(b, tcell.ColorRed), layout, false)
	}
	return layout
}

func keyLegend(vs view.ViewState) string {
	defaultKeys := "Browse (↓ ↑), Expand (→) ┊ (enter) log, (b)rowser ┊ Tilt (l)og ┊ (q)uit  "
	if vs.LogModal.TiltLog || vs.LogModal.ResourceLogNumber != 0 {
		return "Scroll (↓ ↑) ┊ (esc) close logs "
	} else if vs.AlertMessage != "" {
		return "Tilt (l)og ┊ (esc) close alert "
	}
	return defaultKeys
}

func (r *Renderer) renderFooter(v view.View, keys string) rty.Component {
	footer := rty.NewConcatLayout(rty.DirHor)
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()

	sbLeft.Text(" ") // Indent
	errorCount := 0
	for _, res := range v.Resources {
		if isInError(res) {
			errorCount++
		}
	}
	if errorCount == 0 && v.TiltfileErrorMessage == "" {
		sbLeft.Fg(cGood).Text("✓").Fg(tcell.ColorDefault).Fg(cText).Text(" OK").Fg(tcell.ColorDefault)
	} else {
		var errorCountMessage string
		var tiltfileError strings.Builder
		s := "error"
		if errorCount > 1 {
			s = "errors"
		}

		if errorCount > 0 {
			errorCountMessage = fmt.Sprintf(" %d %s", errorCount, s)
		}

		if v.TiltfileErrorMessage != "" {
			_, err := tiltfileError.WriteString(" • Error executing Tiltfile")
			if err != nil {
				// This space intentionally left blank
			}
		}
		sbLeft.Fg(cBad).Text("✖").Fg(tcell.ColorDefault).Fg(cText).Textf("%s%s", errorCountMessage, tiltfileError.String()).Fg(tcell.ColorDefault)
	}
	sbRight.Fg(cText).Text(keys).Fg(tcell.ColorDefault)

	footer.Add(sbLeft.Build())
	footer.Add(rty.TextString("   ")) // minimum 3 spaces between left and right
	footer.AddDynamic(rty.NewFillerString(' '))
	footer.Add(sbRight.Build())

	return rty.NewFixedSize(rty.Bg(footer, tcell.ColorWhiteSmoke), rty.GROW, 1)
}

func isInError(res view.Resource) bool {
	return res.LastBuildError != "" || podStatusColors[res.PodStatus] == cBad || isCrashing(res)
}

func isCrashing(res view.Resource) bool {
	return res.PodRestarts > 0 ||
		res.LastBuildReason.Has(model.BuildReasonFlagCrash) ||
		res.CurrentBuildReason.Has(model.BuildReasonFlagCrash) ||
		res.PendingBuildReason.Has(model.BuildReasonFlagCrash)
}

func (r *Renderer) renderFullLogModal(v view.View, background rty.Component) rty.Component {
	return r.renderLogModal("TILT LOG", v.Log, background)
}

func bestLogs(res view.Resource) string {
	// ~~ TEMPORARY: just view the logs for a dc resource:
	if res.IsDCManifest {
		if res.LastBuildStartTime.Before(res.PodCreationTime) &&
			len(strings.TrimSpace(res.LastBuildLog)) > 0 {
			return res.LastBuildLog + "\n\n" + res.Log
		}
		return res.Log
	}
	// A build is in progress, triggered by an explicit edit.
	if res.CurrentBuildStartTime.After(res.LastBuildFinishTime) &&
		!res.CurrentBuildReason.IsCrashOnly() {
		return res.CurrentBuildLog
	}

	// The last build was an error.
	if res.LastBuildError != "" {
		return res.LastBuildLog
	}

	// The last build finished, but the pod hasn't started yet.
	if res.LastBuildStartTime.After(res.PodCreationTime) {
		return res.LastBuildLog
	}

	// The last build finished, so prepend them to pod logs.
	if res.LastBuildStartTime.Before(res.PodCreationTime) &&
		len(strings.TrimSpace(res.LastBuildLog)) > 0 {
		return res.LastBuildLog + "\n\n" + res.Log
	}

	return res.Log
}

func (r *Renderer) renderResourceLogModal(res view.Resource, background rty.Component) rty.Component {
	s := bestLogs(res)
	if len(strings.TrimSpace(s)) == 0 {
		s = fmt.Sprintf("No log output for %s", res.Name)
	}

	return r.renderLogModal(fmt.Sprintf("LOG: %s", res.Name), s, background)
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
	l.Add(r.renderTiltfileError(v))

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
	if l := r.resourceDC(res, rv); l != nil {
		layout.Add(l)
	}
	// TODO: should be able to add either k8s logs or DC logs
	layout.Add(r.resourceK8sLogs(res, rv))
	layout.Add(r.resourceTilt(res, rv))
	return layout
}

func (r *Renderer) renderTiltfileError(v view.View) rty.Component {
	if v.TiltfileErrorMessage != "" {
		c := rty.NewConcatLayout(rty.DirVert)
		c.Add(rty.TextString("Error executing Tiltfile:"))
		c.Add(rty.TextString(v.TiltfileErrorMessage))
		c.Add(rty.NewFillerString('—'))
		return c
	}

	return rty.NewLines()
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
		sbRight.Textf(" OK • %s ago", formatDeployAge(time.Since(res.LastDeployTime)))
	}

	l.Add(sbLeft.Build())
	l.Add(rty.Fg(rty.NewFillerString('╌'), cLightText))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceK8s(res view.Resource, rv view.ResourceViewState) rty.Component {
	if res.IsYAMLManifest || res.IsDCManifest {
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

	podStatusColor, ok := podStatusColors[res.PodStatus]
	if !ok {
		podStatusColor = cLightText
	}

	if isCrashing(res) {
		podStatusColor = cBad
	}

	sbLeft.Fg(podStatusColor).Textf("%s●  ", indent).Fg(tcell.ColorDefault)

	if res.PodStatus != "" {
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
		sbRight.Textf(" %s", formatDeployAge(time.Since(res.PodCreationTime)))
	}

	sbLeft.Fg(cLightText).Textf("K8S: ").Fg(tcell.ColorDefault).Text(status)

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceDC(res view.Resource, rv view.ResourceViewState) rty.Component {
	if !res.IsDCManifest {
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

	dcStatusColor := cLightText
	if res.DCState == dockercompose.StateInProg {
		dcStatusColor = cPending
	} else if res.DCState == dockercompose.StateUp {
		dcStatusColor = cGood
	} else if res.DCState == dockercompose.StateDown {
		dcStatusColor = cBad
	}

	sbLeft.Fg(dcStatusColor).Textf("%s●  ", indent).Fg(tcell.ColorDefault)

	if res.DCState != "" {
		status = res.DCState
	}

	sbLeft.Fg(cLightText).Textf("DC: ").Fg(tcell.ColorDefault).Text(status)

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	return l
}

type buildStatus struct {
	status      string
	statusColor tcell.Color
	editsPrefix string
	edits       string
	duration    string
}

func (r *Renderer) resourceTilt(res view.Resource, rv view.ResourceViewState) rty.Component {
	lines := rty.NewLines()
	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()

	indent := strings.Repeat(" ", 8)

	bs := r.makeBuildStatus(res)

	sbLeft.Fg(bs.statusColor).Textf("%s●", indent).Fg(tcell.ColorDefault)
	sbLeft.Fg(cLightText).Text(" TILT: ").Fg(tcell.ColorDefault).Text(bs.status)

	sbLeft.Fg(cLightText).Text(bs.editsPrefix).Fg(tcell.ColorDefault).Text(bs.edits)
	sbRight.Fg(cLightText).Text("DURATION ")
	sbRight.Fg(tcell.ColorDefault).Textf("%s           ", bs.duration) // Last char cuts off

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	lines.Add(l)
	lines.Add(r.lastBuildLogs(res, rv))
	return lines
}

func (r *Renderer) makeBuildStatus(res view.Resource) buildStatus {
	bs := buildStatus{
		status:      r.spinner(),
		statusColor: cPending,
		duration:    formatBuildDuration(res.LastBuildDuration),
	}

	if !res.LastBuildFinishTime.IsZero() {
		if res.LastBuildError != "" {
			bs.statusColor = cBad
			bs.status = "ERROR"
		} else {
			bs.statusColor = cGood
			bs.status = "OK"
		}
	}

	if !res.LastDeployTime.IsZero() {
		if len(res.LastDeployEdits) > 0 {
			bs.editsPrefix = " • EDITS "
			bs.edits = formatFileList(res.LastDeployEdits)
		}
	}

	if !res.CurrentBuildStartTime.IsZero() && !res.CurrentBuildReason.IsCrashOnly() {
		bs = buildStatus{
			status:      "In Progress",
			statusColor: cPending,
			duration:    formatBuildDuration(time.Since(res.CurrentBuildStartTime)),
		}
		if len(res.CurrentBuildEdits) > 0 {
			bs.editsPrefix = " • EDITS "
			bs.edits = formatFileList(res.CurrentBuildEdits)
		}
	}

	if !res.PendingBuildSince.IsZero() && !res.PendingBuildReason.IsCrashOnly() {
		bs = buildStatus{
			statusColor: cPending,
			status:      "Pending",
			duration:    formatBuildDuration(time.Since(res.PendingBuildSince)),
		}
		if len(res.PendingBuildEdits) > 0 {
			bs.editsPrefix = " • EDITS "
			bs.edits = formatFileList(res.PendingBuildEdits)
		}
	}

	return bs
}

func (r *Renderer) resourceK8sLogs(res view.Resource, rv view.ResourceViewState) rty.Component {
	lh := rty.NewConcatLayout(rty.DirHor)
	lv := rty.NewConcatLayout(rty.DirVert)
	spacer := rty.TextString(strings.Repeat(" ", 12))

	needsSpacer := false
	if !rv.IsCollapsed {
		if isCrashing(res) {
			podLog := res.Log
			if podLog == "" {
				podLog = res.CrashLog
			}
			abbrevLog := abbreviateLog(podLog)
			for _, logLine := range abbrevLog {
				lv.Add(rty.TextString(logLine))
				needsSpacer = true
			}
		}
	}

	if needsSpacer {
		lh.Add(spacer)
	}
	lh.AddDynamic(lv)

	return lh
}

func (r *Renderer) lastBuildLogs(res view.Resource, rv view.ResourceViewState) rty.Component {
	lh := rty.NewConcatLayout(rty.DirHor)
	lv := rty.NewConcatLayout(rty.DirVert)
	spacer := rty.TextString(strings.Repeat(" ", 12))
	needsSpacer := false

	if !rv.IsCollapsed {
		if res.LastBuildError != "" {
			abbrevLog := abbreviateLog(res.LastBuildLog)
			for _, logLine := range abbrevLog {
				lv.Add(rty.TextString(logLine))
				needsSpacer = true
			}

			// if the build log is non-empty, it will contain the error, so we don't need to show this separately
			if len(abbrevLog) == 0 {
				lv.Add(rty.TextString(fmt.Sprintf("Error: %s", res.LastBuildError)))
				needsSpacer = true
			}
		}
	}

	if needsSpacer {
		lh.Add(spacer)
	}
	lh.AddDynamic(lv)

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
