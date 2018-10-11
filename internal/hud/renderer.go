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

func renderResource(p *pen, r view.Resource) {
	cLightText := tcell.StyleDefault.Foreground(tcell.Color241)
	// cGreen := tcell.StyleDefault.Foreground(tcell.Color64)
	// cRed := tcell.StyleDefault.Foreground(tcell.Color124)
	// cYellow := tcell.StyleDefault.Foreground(tcell.Color136)

	// Resource Title ---------------------------------------
	deployString := "not deployed yet"
	if !r.LastDeployTime.Equal(time.Time{}) {
		deployString = fmt.Sprintf("deployed %s ago", formatDuration(time.Since(r.LastDeployTime)))
	}
	p.puts(r.Name)
	p.style = cLightText
	dashSize := 35
	p.putsf(" %s ", strings.Repeat("┄", dashSize-len(r.Name)))
	p.style = tcell.StyleDefault
	p.puts(deployString)
	p.newln()

	// Resource FS Changes ---------------------------------------
	p.style = cLightText
	p.putsf("  (Watching %s/)", r.DirectoryWatched)
	p.style = tcell.StyleDefault
	if !r.LastDeployTime.Equal(time.Time{}) {
		if len(r.LastDeployEdits) > 0 {
			p.style = cLightText
			p.puts(" Last Deployed Edits: ")
			p.style = tcell.StyleDefault
			p.puts(formatFileList(r.LastDeployEdits))
		}
	}
	p.newln()

	// Build Info ---------------------------------------
	var buildStrings []string

	if !r.CurrentBuildStartTime.Equal(time.Time{}) {
		s := fmt.Sprintf("In Progress - For %s", formatDuration(time.Since(r.CurrentBuildStartTime)))
		if len(r.CurrentBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.CurrentBuildEdits))
		}
		buildStrings = append(buildStrings, s)
	}

	if !r.PendingBuildSince.Equal(time.Time{}) {
		s := fmt.Sprintf("Pending - For %s", formatDuration(time.Since(r.PendingBuildSince)))
		if len(r.PendingBuildEdits) > 0 {
			s += fmt.Sprintf(" • Edits: %s", formatFileList(r.PendingBuildEdits))
		}
		buildStrings = append(buildStrings, s)
	}

	if !r.LastBuildFinishTime.Equal(time.Time{}) {
		shortBuildStatus := "OK"
		if r.LastBuildError != "" {
			shortBuildStatus = "ERR"
		}

		s := fmt.Sprintf("Last build (done in %s) ended %s ago — %s",
			formatPreciseDuration(r.LastBuildDuration),
			formatDuration(time.Since(r.LastBuildFinishTime)),
			shortBuildStatus)

		buildStrings = append(buildStrings, s)

		if r.LastBuildError != "" {
			s := fmt.Sprintf("Error: %s", r.LastBuildError)
			buildStrings = append(buildStrings, s)
		}
	}

	if len(buildStrings) == 0 {
		buildStrings = []string{"no build yet"}
	}
	p.style = cLightText

	p.style = cLightText
	p.puts("  BUILD: ")
	p.style = tcell.StyleDefault
	p.puts(buildStrings[0])
	p.newln()
	for _, s := range buildStrings[1:] {
		p.putlnf("         %s", s)
	}

	// Kubernetes Info ---------------------------------------
	if r.PodStatus != "" {
		p.style = cLightText
		p.puts("    K8S: ")
		p.style = tcell.StyleDefault
		p.putsf("Pod [%s] • %s ago — %s", r.PodName, formatDuration(time.Since(r.PodCreationTime)), r.PodStatus)
	}
	if r.Endpoint != "" {
		p.putlnf("         %s", r.Endpoint)
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
