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
}

func NewRenderer() *Renderer {
	return &Renderer{
		mu: new(sync.Mutex),
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

	split.Add(r.renderResources(v.Resources))
	l.Add(split)

	return l
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

func (r *Renderer) renderResources(rs []view.Resource) rty.Component {
	childNames := make([]string, len(rs))
	for i, r := range rs {
		childNames[i] = r.Name
	}

	l, selectedResource := r.rty.RegisterElementScroll("resources", childNames)

	for _, r := range rs {
		l.Add(renderResource(r, selectedResource == r.Name))
	}

	return l
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func spinner() string {
	return spinnerChars[time.Now().Second()%len(spinnerChars)]
}

func renderResource(r view.Resource, selected bool) rty.Component {
	layout := rty.NewConcatLayout(rty.DirVert)

	sb := rty.NewStringBuilder()
	if selected {
		sb.Text("▶ ")
	} else {
		sb.Text("  ")
	}
	sb.Text(r.Name)
	const dashSize = 35
	sb.Fg(cLightText).Textf(" %s ", strings.Repeat("┄", dashSize-len(r.Name))).Fg(tcell.ColorDefault)
	if r.LastDeployTime.Equal(time.Time{}) {
		sb.Text("not deployed yet")
	} else {
		sb.Textf("deployed %s ago", formatDuration(time.Since(r.LastDeployTime)))
	}

	layout.Add(sb.Build())

	if len(r.DirectoriesWatched) > 0 {
		var dirs []string
		for _, s := range r.DirectoriesWatched {
			dirs = append(dirs, fmt.Sprintf("%s/", s))
		}
		sb := rty.NewStringBuilder()
		sb.Fg(cLightText).Textf("  (Watching %s)", strings.Join(dirs, " ")).Fg(tcell.ColorDefault)
		layout.Add(sb.Build())
	}

	if !r.LastDeployTime.Equal(time.Time{}) {
		if len(r.LastDeployEdits) > 0 {
			sb := rty.NewStringBuilder()
			sb.Fg(cLightText).Text(" Last Deployed Edits: ").Fg(tcell.ColorDefault)
			sb.Text(formatFileList(r.LastDeployEdits))
			layout.Add(sb.Build())
		}
	}

	// Build Info ---------------------------------------
	var buildComponents []rty.Component

	if !r.CurrentBuildStartTime.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()
		sb.Fg(cPending).Textf("In Progress %s", spinner()).Fg(tcell.ColorDefault)
		sb.Textf(" - For %s", formatDuration(time.Since(r.CurrentBuildStartTime)))
		if len(r.CurrentBuildEdits) > 0 {
			sb.Textf(" • Edits: %s", formatFileList(r.CurrentBuildEdits))
		}
		buildComponents = append(buildComponents, sb.Build())
	}

	if !r.PendingBuildSince.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()
		sb.Fg(cPending).Text("Pending").Fg(tcell.ColorDefault)
		sb.Textf(" - For %s", formatDuration(time.Since(r.PendingBuildSince)))
		if len(r.PendingBuildEdits) > 0 {
			sb.Textf(" • Edits: %s", formatFileList(r.PendingBuildEdits))
		}
		buildComponents = append(buildComponents, sb.Build())
	}

	if !r.LastBuildFinishTime.Equal(time.Time{}) {
		sb := rty.NewStringBuilder()

		sb.Textf("Last build (done in %s) ended %s ago — ",
			formatPreciseDuration(r.LastBuildDuration),
			formatDuration(time.Since(r.LastBuildFinishTime)))

		if r.LastBuildError != "" {
			sb.Fg(cBad).Text("ERR")
		} else {
			sb.Fg(cGood).Text("OK")
		}
		sb.Fg(tcell.ColorDefault)

		buildComponents = append(buildComponents, sb.Build())

		if r.LastBuildError != "" {
			s := fmt.Sprintf("Error: %s", r.LastBuildError)
			buildComponents = append(buildComponents, rty.TextString(s))
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
	if r.PodStatus != "" {
		podStatusColor, ok := podStatusColors[r.PodStatus]
		if !ok {
			podStatusColor = tcell.ColorDefault
		}

		sb := rty.NewStringBuilder()
		sb.Fg(cLightText).Text("    K8S: ").Fg(tcell.ColorDefault)
		sb.Textf("Pod [%s] • %s ago — ", r.PodName, formatDuration(time.Since(r.PodCreationTime)))
		sb.Fg(podStatusColor).Text(r.PodStatus).Fg(tcell.ColorDefault)

		// TODO(maia): show # restarts even if == 0 (in gray or green)?
		if r.PodRestarts > 0 {
			sb.Fg(cBad).Textf(" [%d restart(s)]", r.PodRestarts).Fg(tcell.ColorDefault)
		}

		layout.Add(sb.Build())
	}

	if len(r.Endpoints) != 0 {
		sb := rty.NewStringBuilder()
		sb.Textf("         %s", strings.Join(r.Endpoints, " "))
		layout.Add(sb.Build())
	}

	layout.Add(rty.NewLine())

	return layout
}

func (r *Renderer) SetUp(event ReadyEvent) (chan tcell.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// TODO(maia): support sigwinch
	// TODO(maia): pass term name along with ttyPath via RPC. Temporary hack:
	// get termName from current terminal, assume it's the same 🙈
	screen, err := tcell.NewScreenFromTty(event.ttyPath, nil, os.Getenv("TERM"))
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
