package hud

import (
	"fmt"
	"os"
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

func (r *Renderer) Render(v view.View) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rty != nil {
		layout := r.layout(v)
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

	return fmt.Sprintf("%ds", int(d.Seconds()))
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

var cLightText = tcell.Color241
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

func (r *Renderer) layout(v view.View) rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)
	if v.ViewState.ShowNarration {
		l.Add(renderNarration(v.ViewState.NarrationMessage))
		l.Add(rty.NewLine())
	}

	split := rty.NewFlexLayout(rty.DirHor)

	split.Add(r.renderResources(v))
	l.Add(split)

	if v.ViewState.DisplayedLogNumber != 0 {
		return r.renderLogModal(v.Resources[v.ViewState.DisplayedLogNumber-1], l)
	} else {
		return l
	}
}

func (r *Renderer) renderLogModal(res view.Resource, background rty.Component) rty.Component {
	var s string
	if res.LastBuildError != "" && len(strings.TrimSpace(res.LastBuildLog)) > 0 {
		s = res.LastBuildLog
	} else if len(strings.TrimSpace(res.PodLog)) > 0 {
		s = res.PodLog
	} else {
		s = fmt.Sprintf("No log output for %s", res.Name)
	}
	sl := rty.NewTextScrollLayout(logScrollerName)
	sl.Add(rty.TextString(s))
	box := rty.NewBox()
	box.SetInner(sl)
	l := rty.NewFlexLayout(rty.DirVert)
	l.Add(box)
	l.Add(rty.NewStringBuilder().Bg(tcell.ColorBlue).Text("Press <Enter> to stop viewing log").Build())

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

	box := rty.Fg(rty.Bg(lines, tcell.ColorLightGrey), tcell.ColorBlack)
	return rty.NewFixedSize(box, rty.GROW, 3)
}

func (r *Renderer) renderResources(v view.View) rty.Component {
	rs := v.Resources
	childNames := make([]string, len(rs))
	for i, r := range rs {
		childNames[i] = r.Name
	}

	l, selectedResource := r.rty.RegisterElementScroll(resourcesScollerName, childNames)

	for i, res := range rs {
		l.Add(r.renderResource(res, v.ViewState.Resources[i], selectedResource == res.Name))
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

	sb := rty.NewStringBuilder()
	p := "  "
	if selected {
		p = "▶ "
	}
	if selected && rv.IsExpanded {
		p = "▼ "
	}
	sb.Text(p)

	sb.Text(res.Name)
	const dashSize = 35
	sb.Fg(cLightText).Textf(" %s ", strings.Repeat("┄", dashSize-len(res.Name))).Fg(tcell.ColorDefault)
	if res.LastDeployTime.Equal(time.Time{}) {
		sb.Text("not deployed yet")
	} else {
		sb.Textf("deployed %s ago", formatDuration(time.Since(res.LastDeployTime)))
	}

	layout.Add(sb.Build())

	if len(res.DirectoriesWatched) > 0 {
		var dirs []string
		for _, s := range res.DirectoriesWatched {
			dirs = append(dirs, fmt.Sprintf("%s/", s))
		}
		sb := rty.NewStringBuilder()
		sb.Fg(cLightText).Textf("  (Watching %s)", strings.Join(dirs, " ")).Fg(tcell.ColorDefault)
		layout.Add(sb.Build())
	}

	if !res.LastDeployTime.Equal(time.Time{}) {
		if len(res.LastDeployEdits) > 0 {
			sb := rty.NewStringBuilder()
			sb.Fg(cLightText).Text("  Last Deployed Edits: ").Fg(tcell.ColorDefault)
			sb.Text(formatFileList(res.LastDeployEdits))
			layout.Add(sb.Build())
		}
	}

	// Build Info ---------------------------------------
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

	if !res.LastBuildFinishTime.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()

		sb.Textf("Last build (done in %s) ended %s ago — ",
			formatPreciseDuration(res.LastBuildDuration),
			formatDuration(time.Since(res.LastBuildFinishTime)))

		if res.LastBuildError != "" {
			sb.Fg(cBad).Text("ERR")
		} else {
			sb.Fg(cGood).Text("OK")
		}
		sb.Fg(tcell.ColorDefault)

		buildComponents = append(buildComponents, sb.Build())

		if rv.IsExpanded {
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

	// Kubernetes Info ---------------------------------------
	if res.PodStatus != "" {
		podStatusColor, ok := podStatusColors[res.PodStatus]
		if !ok {
			podStatusColor = tcell.ColorDefault
		}

		sb := rty.NewStringBuilder()
		sb.Fg(cLightText).Text("    K8S: ").Fg(tcell.ColorDefault)
		sb.Textf("Pod [%s] • %s ago — ", res.PodName, formatDuration(time.Since(res.PodCreationTime)))
		sb.Fg(podStatusColor).Text(res.PodStatus).Fg(tcell.ColorDefault)

		// TODO(maia): show # restarts even if == 0 (in gray or green)?
		if res.PodRestarts > 0 {
			sb.Fg(cBad).Textf(" [%d restart(s)]", res.PodRestarts).Fg(tcell.ColorDefault)
		}

		layout.Add(sb.Build())

		if len(res.Endpoints) != 0 {
			sb := rty.NewStringBuilder()
			sb.Textf("         %s", strings.Join(res.Endpoints, " "))
			layout.Add(sb.Build())
		}

		if res.PodRestarts > 0 {
			logLines := abbreviateLog(res.PodLog)
			if len(logLines) > 0 {
				layout.Add(rty.NewStringBuilder().Text("    ").Fg(cLightText).Text("LOG:").Fg(tcell.ColorDefault).Textf(" %s", logLines[0]).Build())
				for _, logLine := range logLines[1:] {
					layout.Add(rty.TextString(fmt.Sprintf("         %s", logLine)))
				}
			}
		}
	}

	layout.Add(rty.NewLine())

	return layout
}

func (r *Renderer) SetUp(event ReadyEvent, sigwinch chan os.Signal) (chan tcell.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO(maia): pass term name along with ttyPath via RPC. Temporary hack:
	// get termName from current terminal, assume it's the same 🙈
	screen, err := tcell.NewScreenFromTty(event.ttyPath, sigwinch, os.Getenv("TERM"))
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
