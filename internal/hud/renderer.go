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

	if vs.LogModal.TiltLog {
		return r.renderFullLogModal(v, l)
	} else if vs.LogModal.ResourceLogNumber != 0 {
		return r.renderResourceLogModal(v.Resources[vs.LogModal.ResourceLogNumber-1], l)
	} else {
		return l
	}
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
		sbLeft.Fg(cGood).Text("✓").Fg(tcell.ColorDefault).Text(" OK")
	} else {
		s := "error"
		if errorCount > 1 {
			s = "errors"
		}
		sbLeft.Fg(cBad).Text("✖").Fg(tcell.ColorDefault).Textf(" %d %s", errorCount, s)
	}
	sbRight.Text(keys)

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
	box := rty.NewBox()
	box.SetInner(sl)
	box.SetTitle(title)
	l := rty.NewFlexLayout(rty.DirVert)
	l.Add(box)
	ml := rty.NewModalLayout(background, l, .9)
	return ml
}

func renderNarration(msg string) rty.Component {
	lines := rty.NewLines()
	l := rty.NewLine()
	l.Add(rty.TextString(msg))
	lines.Add(rty.NewLine())
	lines.Add(l)
	lines.Add(rty.NewLine())

	box := rty.Fg(rty.Bg(lines, tcell.ColorLightGrey), tcell.ColorDefault)
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
	layout.Add(resourceTitle(selected, rv, res))
	layout.Add(r.resourceK8s(res, rv))
	layout.Add(r.resourceTilt(res, rv))
	layout.Add(rty.NewLine())
	return layout
}

func resourceTitle(selected bool, rv view.ResourceViewState, res view.Resource) rty.Component {
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
		sbRight.Textf(" OK • %s ago ", formatDuration(time.Since(res.LastDeployTime)))
	}

	l.Add(sbLeft.Build())
	l.Add(rty.Fg(rty.NewFillerString('╌'), cLightText))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceK8s(res view.Resource, rv view.ResourceViewState) rty.Component {
	l := rty.NewLine()
	sbLeft := rty.NewStringBuilder()
	sbRight := rty.NewStringBuilder()
	sbDetail := rty.NewStringBuilder()
	status := r.spinner()
	indent := "    "

	if res.PodStatus != "" {
		podStatusColor, ok := podStatusColors[res.PodStatus]
		if !ok {
			podStatusColor = tcell.ColorDefault
		}
		sbLeft.Fg(podStatusColor).Textf("%s● ", indent).Fg(tcell.ColorDefault)
		status = res.PodStatus

		if len(res.Endpoints) != 0 {
			sbRight.Textf("%s • ", strings.Join(res.Endpoints, " "))
		}

		// TODO(maia): show # restarts even if == 0 (in gray or green)?
		if res.PodRestarts > 0 {
			sbRight.Fg(cBad).Textf("%d restart(s) • ", res.PodRestarts).Fg(tcell.ColorDefault)
		}

		sbRight.Textf("%s ago ", formatDuration(time.Since(res.PodCreationTime)))

		// K8s Log
		if !rv.IsCollapsed {
			if res.PodRestarts > 0 {
				logLines := abbreviateLog(res.PodLog)
				if len(logLines) > 0 {
					layout.Add(rty.NewStringBuilder().Text(indent).Fg(cLightText).Text("LOG:").Fg(tcell.ColorDefault).Textf(" %s", logLines[0]).Build())
					for _, logLine := range logLines[1:] {
						layout.Add(rty.TextString(fmt.Sprintf("%s%s", indent, logLine)))
					}
				}
			}
		}
	} else {
		sbLeft.Fg(cLightText).Textf("%s● ", indent).Fg(tcell.ColorDefault)
	}

	sbLeft.Fg(cLightText).Textf("K8S: ").Fg(tcell.ColorDefault).Text(status)

	l.Add(sbLeft.Build())
	l.Add(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	return l
}

func (r *Renderer) resourceTilt(res view.Resource, rv view.ResourceViewState) rty.Component {
	// Last Deployed Edits
	if !res.LastDeployTime.Equal(time.Time{}) {
		if len(res.LastDeployEdits) > 0 {
			sb := rty.NewStringBuilder()
			sb.Fg(cLightText).Text("  Last Deployed Edits: ").Fg(tcell.ColorDefault)
			sb.Text(formatFileList(res.LastDeployEdits))
			layout.Add(sb.Build())
		}
	}

	var buildComponents []rty.Component
	if !res.CurrentBuildStartTime.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()
		sb.Fg(cPending).Textf("In Progress %s", r.spinner()).Fg(tcell.ColorDefault)
		sb.Textf(" - For %s", formatDuration(time.Since(res.CurrentBuildStartTime)))
		if len(res.CurrentBuildEdits) > 0 {
			sb.Textf(" • Edits: %s", formatFileList(res.CurrentBuildEdits))
		}
		buildComponents = append(buildComponents, sb.Build())
	}
	if !res.PendingBuildSince.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()
		sb.Fg(cPending).Text("Pending").Fg(tcell.ColorDefault)
		sb.Textf(" - For %s", formatDuration(time.Since(res.PendingBuildSince)))
		if len(res.PendingBuildEdits) > 0 {
			sb.Textf(" • Edits: %s", formatFileList(res.PendingBuildEdits))
		}
		buildComponents = append(buildComponents, sb.Build())
	}

	if res.LastManifestLoadError != "" {
		sb := rty.NewStringBuilder()
		sb.Textf("Failed to load manifest - ").Fg(cBad).Text("ERR").Fg(tcell.ColorDefault)
		buildComponents = append(buildComponents, sb.Build())

		sb = rty.NewStringBuilder().Text(res.LastManifestLoadError)
		buildComponents = append(buildComponents, sb.Build())
	} else if !res.LastBuildFinishTime.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()

		sb.Textf("Last build ended %s ago (took %s) — ",
			formatDuration(time.Since(res.LastBuildFinishTime)),
			formatPreciseDuration(res.LastBuildDuration))

		if res.LastBuildError != "" {
			sb.Fg(cBad).Text("ERR")
		} else {
			sb.Fg(cGood).Text("OK")
		}
		sb.Fg(tcell.ColorDefault)

		buildComponents = append(buildComponents, sb.Build())

		if !rv.IsCollapsed {
			if res.LastBuildError != "" {
				abbrevLog := abbreviateLog(res.LastBuildLog)
				for _, logLine := range abbrevLog {
					buildComponents = append(buildComponents, rty.TextString(logLine))
				}

				// if the build log is non-empty, it will contain the error, so we don't need to show this separately
				if len(abbrevLog) == 0 {
					buildComponents = append(buildComponents, rty.TextString(fmt.Sprintf("Error: %s", res.LastBuildError)))
				}
			}
		}
	}
	if len(buildComponents) == 0 {
		buildComponents = []rty.Component{rty.TextString("no build yet")}
	}
	l := rty.NewLine()
	l.Add(rty.ColoredString("  BUILD: ", cLightText))
	l.Add(buildComponents[0])
	layout.Add(l)
	for _, c := range buildComponents[1:] {
		l := rty.NewLine()
		l.Add(rty.TextString("         "))
		l.Add(c)
		layout.Add(l)
	}
	layout.Add(rty.NewLine())
	return l
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
