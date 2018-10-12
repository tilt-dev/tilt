package hud

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/windmilleng/tcell"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/store"
)

type Renderer struct {
	screen tcell.Screen

	screenMu *sync.Mutex
}

func NewRenderer() *Renderer {
	return &Renderer{
		screenMu: new(sync.Mutex),
	}
}

func (r *Renderer) Render(v view.View) error {
	r.screenMu.Lock()
	defer r.screenMu.Unlock()
	if r.screen != nil {
		r.screen.Clear()
		p := newPen(r.screen)
		for _, res := range v.Resources {
			renderResource(p, res)
		}
		r.screen.Show()
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

var cLightText = tcell.StyleDefault.Foreground(tcell.Color241)
var cGood = tcell.StyleDefault.Foreground(tcell.ColorGreen)
var cBad = tcell.StyleDefault.Foreground(tcell.ColorRed)
var cPending = tcell.StyleDefault.Foreground(tcell.ColorYellow)

var podStatusColors = map[string]tcell.Style{
	"Running":           cGood,
	"ContainerCreating": cPending,
	"Error":             cBad,
	"CrashLoopBackOff":  cBad,
}

func renderResource(p *pen, r view.Resource) {

	// Resource Title ---------------------------------------
	deployString := "not deployed yet"
	if !r.LastDeployTime.Equal(time.Time{}) {
		deployString = fmt.Sprintf("deployed %s ago", formatDuration(time.Since(r.LastDeployTime)))
	}
	p.puts(r.Name)
	const dashSize = 35
	p.putStyledString(styledString{fmt.Sprintf(" %s ", strings.Repeat("┄", dashSize-len(r.Name))), cLightText})
	p.puts(deployString)

	// Resource FS Changes ---------------------------------------
	if len(r.DirectoriesWatched) > 0 {
		p.newln()
		var dirs []string
		for _, s := range r.DirectoriesWatched {
			dirs = append(dirs, fmt.Sprintf("%s/", s))
		}
		p.putlnStyledString(styledString{fmt.Sprintf("  (Watching %s)", strings.Join(dirs, " ")), cLightText})
	}

	if !r.LastDeployTime.Equal(time.Time{}) {
		if len(r.LastDeployEdits) > 0 {
			p.putStyledString(styledString{" Last Deployed Edits: ", cLightText})
			p.puts(formatFileList(r.LastDeployEdits))
		}
	}

	// Build Info ---------------------------------------
	var buildStrings [][]styledString

	if !r.CurrentBuildStartTime.Equal(time.Time{}) {
		statusString := styledString{"In Progress", cPending}
		s := fmt.Sprintf(" - For %s", formatDuration(time.Since(r.CurrentBuildStartTime)))
		if len(r.CurrentBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.CurrentBuildEdits))
		}
		buildStrings = append(buildStrings, []styledString{statusString, {string: s}})
	}

	if !r.PendingBuildSince.Equal(time.Time{}) {
		statusString := styledString{"Pending", cPending}
		s := fmt.Sprintf(" - For %s", formatDuration(time.Since(r.PendingBuildSince)))
		if len(r.PendingBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.PendingBuildEdits))
		}
		buildStrings = append(buildStrings, []styledString{statusString, {string: s}})
	}

	if !r.LastBuildFinishTime.Equal(time.Time{}) {
		shortBuildStatus := styledString{"OK", cGood}
		if r.LastBuildError != "" {
			shortBuildStatus = styledString{"ERR", cBad}
		}

		s := fmt.Sprintf("Last build (done in %s) ended %s ago — ",
			formatPreciseDuration(r.LastBuildDuration),
			formatDuration(time.Since(r.LastBuildFinishTime)))

		buildStrings = append(buildStrings, []styledString{{string: s}, shortBuildStatus})

		if r.LastBuildError != "" {
			s := fmt.Sprintf("Error: %s", r.LastBuildError)
			buildStrings = append(buildStrings, []styledString{{string: s}})
		}
	}

	if len(buildStrings) == 0 {
		buildStrings = [][]styledString{{{string: "no build yet"}}}
	}
	p.putStyledString(styledString{"  BUILD: ", cLightText})
	p.putlnStyledString(buildStrings[0]...)
	for _, s := range buildStrings[1:] {
		p.puts("         ")
		p.putlnStyledString(s...)
	}

	// Kubernetes Info ---------------------------------------
	if r.PodStatus != "" {
		podStatusColor, ok := podStatusColors[r.PodStatus]
		if !ok {
			podStatusColor = tcell.StyleDefault
		}
		p.putlnStyledString(
			styledString{"    K8S: ", cLightText},
			styledString{string: fmt.Sprintf("Pod [%s] • %s ago — ", r.PodName, formatDuration(time.Since(r.PodCreationTime)))},
			styledString{r.PodStatus, podStatusColor},
		)
	}

	if len(r.Endpoints) != 0 {
		p.putlnf("         %s", strings.Join(r.Endpoints, " "))
	}
	p.newln()
	p.newln()
}

func (r *Renderer) SetUp(event ReadyEvent, st *store.Store) error {
	r.screenMu.Lock()
	defer r.screenMu.Unlock()

	// TODO(maia): support sigwinch
	// TODO(maia): pass term name along with ttyPath via RPC. Temporary hack:
	// get termName from current terminal, assume it's the same 🙈
	screen, err := tcell.NewScreenFromTty(event.ttyPath, nil, os.Getenv("TERM"))
	if err != nil {
		return err
	}
	if err = screen.Init(); err != nil {
		return err
	}
	go func() {
		for {
			ev := screen.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape, tcell.KeyEnter:
					// TODO: tell `tilt hud` to exit
					screen.Fini()
				case tcell.KeyRune:
					switch r := ev.Rune(); {
					case r >= '1' && r <= '9':
						st.Dispatch(NewReplayBuildLogAction(int(r - '0')))
					}
				}
			}
		}
	}()

	r.screen = screen

	return nil
}

func (r *Renderer) Reset() {
	r.screenMu.Lock()
	defer r.screenMu.Unlock()

	r.screen.Fini()
	r.screen = nil
}
