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
	lines := rty.NewLines()
	l := rty.NewLine()
	l.Add(rty.TextString(r.Name))
	const dashSize = 35
	l.Add(rty.ColoredString(fmt.Sprintf(" %s ", strings.Repeat("┄", dashSize-len(r.Name))), cLightText))
	deployString := "not deployed yet"
	if !r.LastDeployTime.Equal(time.Time{}) {
		deployString = fmt.Sprintf("deployed %s ago", formatDuration(time.Since(r.LastDeployTime)))
	}
	l.Add(rty.TextString(deployString))

	lines.Add(l)

	if len(r.DirectoriesWatched) > 0 {
		var dirs []string
		for _, s := range r.DirectoriesWatched {
			dirs = append(dirs, fmt.Sprintf("%s/", s))
		}
		l = rty.NewLine()
		l.Add(rty.ColoredString(fmt.Sprintf("  (Watching %s)", strings.Join(dirs, " ")), cLightText))
		lines.Add(l)
	}

	if !r.LastDeployTime.Equal(time.Time{}) {
		if len(r.LastDeployEdits) > 0 {
			l = rty.NewLine()
			l.Add(rty.ColoredString(" Last Deployed Edits: ", cLightText))
			l.Add(rty.TextString(formatFileList(r.LastDeployEdits)))
			lines.Add(l)
		}
	}

	// Build Info ---------------------------------------
	var buildComponents [][]rty.Component

	if !r.CurrentBuildStartTime.Equal(time.Time{}) {
		statusString := rty.ColoredString(fmt.Sprintf("In Progress %s", spinner()), cPending)
		s := fmt.Sprintf(" - For %s", formatDuration(time.Since(r.CurrentBuildStartTime)))
		if len(r.CurrentBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.CurrentBuildEdits))
		}
		buildComponents = append(buildComponents, []rty.Component{statusString, rty.TextString(s)})
	}

	if !r.PendingBuildSince.Equal(time.Time{}) {
		statusString := rty.ColoredString("Pending", cPending)
		s := fmt.Sprintf(" - For %s", formatDuration(time.Since(r.PendingBuildSince)))
		if len(r.PendingBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.PendingBuildEdits))
		}
		buildComponents = append(buildComponents, []rty.Component{statusString, rty.TextString(s)})
	}

	if !r.LastBuildFinishTime.Equal(time.Time{}) {
		shortBuildStatus := rty.ColoredString("OK", cGood)
		if r.LastBuildError != "" {
			shortBuildStatus = rty.ColoredString("ERR", cBad)
		}

		s := fmt.Sprintf("Last build (done in %s) ended %s ago — ",
			formatPreciseDuration(r.LastBuildDuration),
			formatDuration(time.Since(r.LastBuildFinishTime)))

		buildComponents = append(buildComponents, []rty.Component{rty.TextString(s), shortBuildStatus})

		if r.LastBuildError != "" {
			s := fmt.Sprintf("Error: %s", r.LastBuildError)
			buildComponents = append(buildComponents, []rty.Component{rty.TextString(s)})
		}
	}

	if len(buildComponents) == 0 {
		buildComponents = [][]rty.Component{{rty.TextString("no build yet")}}
	}

	l = rty.NewLine()
	l.Add(rty.ColoredString("  BUILD: ", cLightText))
	for _, c := range buildComponents[0] {
		l.Add(c)
	}

	lines.Add(l)

	for _, cs := range buildComponents[1:] {
		l := rty.NewLine()
		l.Add(rty.TextString("         "))
		for _, c := range cs {
			l.Add(c)
		}
		lines.Add(l)
	}

	// Kubernetes Info ---------------------------------------
	if r.PodStatus != "" {
		podStatusColor, ok := podStatusColors[r.PodStatus]
		if !ok {
			podStatusColor = tcell.ColorDefault
		}

		l := rty.NewLine()
		l.Add(rty.ColoredString("    K8S: ", cLightText))
		l.Add(rty.TextString(fmt.Sprintf("Pod [%s] • %s ago — ", r.PodName, formatDuration(time.Since(r.PodCreationTime)))))
		l.Add(rty.ColoredString(r.PodStatus, podStatusColor))

		// TODO(maia): show # restarts even if == 0 (in gray or green)?
		if r.PodRestarts > 0 {
			l.Add(rty.ColoredString(fmt.Sprintf(" [%d restart(s)]", r.PodRestarts), cBad))
		}
		lines.Add(l)
	}

	if len(r.Endpoints) != 0 {
		l := rty.NewLine()
		l.Add(rty.TextString(fmt.Sprintf("         %s", strings.Join(r.Endpoints, " "))))
		lines.Add(l)
	}

	lines.Add(rty.NewLine())

	return lines
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
