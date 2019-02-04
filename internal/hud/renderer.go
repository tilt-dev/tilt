package hud

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell"
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

var statusColors = map[string]tcell.Color{
	"Running":                          cGood,
	"ContainerCreating":                cPending,
	"Pending":                          cPending,
	"Error":                            cBad,
	"CrashLoopBackOff":                 cBad,
	string(dockercompose.StatusInProg): cPending,
	string(dockercompose.StatusUp):     cGood,
	string(dockercompose.StatusDown):   cBad,
	"Completed":                        cGood,
}

func (r *Renderer) layout(v view.View, vs view.ViewState) rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)
	if vs.ShowNarration {
		l.Add(renderNarration(vs.NarrationMessage))
		l.Add(rty.NewLine())
	}

	l.Add(r.renderResourceHeader(v))
	l.Add(r.renderResources(v, vs))
	l.Add(r.renderPaneHeader(vs))
	l.Add(r.renderLogPane(v, vs))
	l.Add(r.renderFooter(v, keyLegend(v, vs)))

	var ret rty.Component = l
	if vs.LogModal.TiltLog == view.TiltLogFullScreen {
		ret = r.renderTiltLog(v, vs, keyLegend(v, vs), ret)
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

func (r *Renderer) renderLogPane(v view.View, vs view.ViewState) rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	log := rty.NewTextScrollLayout("log")
	log.Add(rty.TextString(v.Log))
	l.Add(log)
	height := 7
	if vs.LogModal.TiltLog == view.TiltLogMinimized {
		height = 1
	}
	return rty.NewFixedSize(l, rty.GROW, height)
}

func (r *Renderer) renderPaneHeader(vs view.ViewState) rty.Component {
	var s string
	switch vs.LogModal.TiltLog {
	case view.TiltLogFullScreen:
		s = "(l) minimize log ↓"
	case view.TiltLogPane:
		s = "(l) maximize log ↑"
	case view.TiltLogMinimized:
		s = "(l) expand log ↑"
	}
	l := rty.NewLine()
	l.Add(rty.NewFillerString('─'))
	l.Add(rty.TextString(fmt.Sprintf(" %s ", s)))
	l.Add(rty.NewFillerString('─'))
	return l
}

func (r *Renderer) renderStatusBar(v view.View) rty.Component {
	sb := rty.NewStringBuilder()
	sb.Text(" ") // Indent
	errorCount := 0
	for _, res := range v.Resources {
		if isInError(res, v.TriggerMode) {
			errorCount++
		}
	}
	if errorCount == 0 && v.TiltfileErrorMessage == "" {
		sb.Fg(cGood).Text("✓").Fg(tcell.ColorDefault).Fg(cText).Text(" OK").Fg(tcell.ColorDefault)
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
			_, err := tiltfileError.WriteString(" • Tiltfile error")
			if err != nil {
				// This space intentionally left blank
			}
		}
		sb.Fg(cBad).Text("✖").Fg(tcell.ColorDefault).Fg(cText).Textf("%s%s", errorCountMessage, tiltfileError.String()).Fg(tcell.ColorDefault)
	}
	return rty.Bg(rty.OneLine(sb.Build()), tcell.ColorWhiteSmoke)
}

func (r *Renderer) renderFooter(v view.View, keys string) rty.Component {
	footer := rty.NewConcatLayout(rty.DirVert)
	footer.Add(r.renderStatusBar(v))
	l := rty.NewConcatLayout(rty.DirHor)
	sbRight := rty.NewStringBuilder()
	sbRight.Text(keys)
	l.AddDynamic(rty.NewFillerString(' '))
	l.Add(sbRight.Build())
	footer.Add(l)
	return rty.NewFixedSize(footer, rty.GROW, 2)
}

func keyLegend(v view.View, vs view.ViewState) string {
	defaultKeys := "Browse (↓ ↑), Expand (→) ┊ (enter) log, (b)rowser ┊ (ctrl-C) quit  "
	if vs.LogModal.TiltLog == view.TiltLogFullScreen {
		return "Scroll (↓ ↑) ┊ cycle (l)og view "
	} else if vs.LogModal.ResourceLogNumber != 0 {
		return "Scroll (↓ ↑) ┊ (esc) close logs "
	} else if vs.AlertMessage != "" {
		return "Tilt (l)og ┊ (esc) close alert "
	} else if v.TriggerMode == model.TriggerManual {
		return "Build (space) ┊ " + defaultKeys
	}
	return defaultKeys
}

func isInError(res view.Resource, triggerMode model.TriggerMode) bool {
	return statusColor(res, triggerMode) == cBad
}

func isCrashing(res view.Resource) bool {
	return (res.IsK8S() && res.K8SInfo().PodRestarts > 0) ||
		res.LastBuild().Reason.Has(model.BuildReasonFlagCrash) ||
		res.CurrentBuild.Reason.Has(model.BuildReasonFlagCrash) ||
		res.PendingBuildReason.Has(model.BuildReasonFlagCrash) ||
		res.IsDC() && res.DockerComposeTarget().Status() == string(dockercompose.StatusCrash)
}

func bestLogs(res view.Resource) string {
	// A build is in progress, triggered by an explicit edit.
	if res.CurrentBuild.StartTime.After(res.LastBuild().FinishTime) &&
		!res.CurrentBuild.Reason.IsCrashOnly() {
		return string(res.CurrentBuild.Log)
	}

	// A build is in progress, triggered by a pod crash.
	if res.CurrentBuild.StartTime.After(res.LastBuild().FinishTime) &&
		res.CurrentBuild.Reason.IsCrashOnly() {
		return res.CrashLog + "\n\n" + string(res.CurrentBuild.Log)
	}

	// The last build was an error.
	if res.LastBuild().Error != nil {
		return string(res.LastBuild().Log)
	}

	if k8sInfo, ok := res.ResourceInfo.(view.K8SResourceInfo); ok {
		// Two cases:
		// 1) The last build finished before this pod started
		// 2) This log is from an in-place container update.
		// in either case, prepend them to pod logs.
		if (res.LastBuild().StartTime.Equal(k8sInfo.PodUpdateStartTime) ||
			res.LastBuild().StartTime.Before(k8sInfo.PodCreationTime)) &&
			len(res.LastBuild().Log) > 0 {
			return string(res.LastBuild().Log) + "\n" + res.ResourceInfo.RuntimeLog()
		}

		// The last build finished, but the pod hasn't started yet.
		if res.LastBuild().StartTime.After(k8sInfo.PodCreationTime) {
			return string(res.LastBuild().Log)
		}
	}

	if res.IsTiltfile {
		return string(res.LastBuild().Log)
	}

	return string(res.LastBuild().Log) + "\n" + res.ResourceInfo.RuntimeLog()
}

func (r *Renderer) renderTiltLog(v view.View, vs view.ViewState, keys string, background rty.Component) rty.Component {
	l := rty.NewConcatLayout(rty.DirVert)
	sl := rty.NewTextScrollLayout(logScrollerName)
	l.Add(r.renderPaneHeader(vs))
	sl.Add(rty.TextString(v.Log))
	l.AddDynamic(sl)
	l.Add(r.renderFooter(v, keys))
	return rty.NewModalLayout(background, l, 1, true)
}

func (r *Renderer) renderResourceLogModal(res view.Resource, background rty.Component) rty.Component {
	s := bestLogs(res)
	if len(strings.TrimSpace(s)) == 0 {
		s = fmt.Sprintf("No log output for %s", res.Name)
	}

	l := rty.NewTextScrollLayout(logScrollerName)
	l.Add(rty.TextString(s))
	box := rty.NewGrowingBox()
	box.SetInner(l)
	box.SetTitle(fmt.Sprintf("LOG: %s", res.Name))

	return rty.NewModalLayout(background, box, 0.9, true)
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

func (r *Renderer) renderResourceHeader(v view.View) rty.Component {
	l := rty.NewConcatLayout(rty.DirHor)
	l.Add(rty.ColoredString("  RESOURCE NAME ", cLightText))
	l.AddDynamic(rty.NewFillerString(' '))

	k8sCell := rty.ColoredString(" DEPLOY", cLightText)
	l.Add(k8sCell)
	l.Add(middotText())

	buildCell := rty.NewMinLengthLayout(BuildDurCellMinWidth+BuildStatusCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(rty.ColoredString("BUILD STATUS", cLightText))
	l.Add(buildCell)
	l.Add(middotText())
	deployCell := rty.NewMinLengthLayout(DeployCellMinWidth+1, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(rty.ColoredString("UPDATED ", cLightText))
	l.Add(deployCell)
	return rty.OneLine(l)
}

func (r *Renderer) renderResources(v view.View, vs view.ViewState) rty.Component {
	rs := v.Resources

	cl := rty.NewConcatLayout(rty.DirVert)

	childNames := make([]string, len(rs))
	for i, r := range rs {
		childNames[i] = r.Name.String()
	}
	// the items added to `l` below must be kept in sync with `childNames` above
	l, selectedResource := r.rty.RegisterElementScroll(resourcesScollerName, childNames)

	if len(rs) > 0 {
		for i, res := range rs {
			l.Add(r.renderResource(res, vs.Resources[i], v.TriggerMode, selectedResource == res.Name.String()))
		}
	}

	cl.Add(l)
	return cl
}

func (r *Renderer) renderResource(res view.Resource, rv view.ResourceViewState, triggerMode model.TriggerMode, selected bool) rty.Component {
	return NewResourceView(res, rv, triggerMode, selected, r.clock).Build()
}

func (r *Renderer) SetUp() (chan tcell.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	screen, err := tcell.NewScreen()
	if err != nil {
		if err == tcell.ErrTermNotFound {
			// The statically-compiled tcell only supports the most common TERM configs.
			// The dynamically-compiled tcell supports more, but has distribution problems.
			// See: https://github.com/gdamore/tcell/issues/252
			term := os.Getenv("TERM")
			return nil, fmt.Errorf("Tilt does not support TERM=%q. "+
				"This is not a common Terminal config. "+
				"If you expect that you're using a common terminal, "+
				"you might have misconfigured $TERM in your .profile.", term)
		}
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
