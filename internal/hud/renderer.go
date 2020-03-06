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
	"github.com/windmilleng/tilt/internal/rty"
	"github.com/windmilleng/tilt/pkg/model"
)

const defaultLogPaneHeight = 8

type Renderer struct {
	rty    rty.RTY
	screen tcell.Screen
	mu     *sync.RWMutex
	clock  func() time.Time
}

func NewRenderer(clock func() time.Time) *Renderer {
	return &Renderer{
		mu:    new(sync.RWMutex),
		clock: clock,
	}
}

func (r *Renderer) Render(v view.View, vs view.ViewState) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rty := r.rty
	if rty != nil {
		layout := r.layout(v, vs)
		rty.Render(layout)
	}
}

var cText = tcell.Color232
var cLightText = tcell.Color243
var cGood = tcell.ColorGreen
var cBad = tcell.ColorRed
var cPending = tcell.Color243

var statusColors = map[string]tcell.Color{
	"Running":                                cGood,
	string(model.RuntimeStatusOK):            cGood,
	string(model.RuntimeStatusNotApplicable): cGood,
	"ContainerCreating":                      cPending,
	"Pending":                                cPending,
	"PodInitializing":                        cPending,
	string(model.RuntimeStatusPending):       cPending,
	"Error":                                  cBad,
	"CrashLoopBackOff":                       cBad,
	"ErrImagePull":                           cBad,
	"ImagePullBackOff":                       cBad,
	"RunContainerError":                      cBad,
	"StartError":                             cBad,
	string(model.RuntimeStatusError):         cBad,
	string(dockercompose.StatusInProg):       cPending,
	string(dockercompose.StatusUp):           cGood,
	string(dockercompose.StatusDown):         cBad,
	"Completed":                              cGood,
}

func (r *Renderer) layout(v view.View, vs view.ViewState) rty.Component {
	l := rty.NewFlexLayout(rty.DirVert)
	if vs.ShowNarration {
		l.Add(renderNarration(vs.NarrationMessage))
		l.Add(rty.NewLine())
	}

	l.Add(r.renderResourceHeader(v))
	l.Add(r.renderResources(v, vs))
	l.Add(r.renderLogPane(v, vs))
	l.Add(r.renderFooter(v, keyLegend(v, vs)))

	var ret rty.Component = l

	ret = r.maybeAddFullScreenLog(v, vs, ret)

	ret = r.maybeAddAlertModal(v, vs, ret)

	return ret
}

func (r *Renderer) maybeAddFullScreenLog(v view.View, vs view.ViewState, layout rty.Component) rty.Component {
	if vs.TiltLogState == view.TiltLogFullScreen {
		tabView := NewTabView(v, vs)

		l := rty.NewConcatLayout(rty.DirVert)
		sl := rty.NewTextScrollLayout("log")
		l.Add(tabView.buildTabs(true))
		sl.Add(rty.TextString(tabView.log()))
		l.AddDynamic(sl)
		l.Add(r.renderFooter(v, keyLegend(v, vs)))

		layout = rty.NewModalLayout(layout, l, 1, true)
	}
	return layout
}

func (r *Renderer) maybeAddAlertModal(v view.View, vs view.ViewState, layout rty.Component) rty.Component {
	alertMsg := ""
	if v.FatalError != nil {
		alertMsg = fmt.Sprintf("Tilt has encountered a fatal error: %s\nOnce you fix this issue you'll need to restart Tilt. In the meantime feel free to browse through the UI.", v.FatalError.Error())
	} else if vs.AlertMessage != "" {
		alertMsg = vs.AlertMessage
	}

	if alertMsg != "" {
		l := rty.NewLines()
		l.Add(rty.TextString(""))

		msg := "   " + alertMsg + "   "
		l.Add(rty.Fg(rty.TextString(msg), tcell.ColorDefault))
		l.Add(rty.TextString(""))

		w := rty.NewWindow(l)
		w.SetTitle("! Alert !")
		layout = r.renderModal(rty.Fg(w, tcell.ColorRed), layout, false)
	}
	return layout
}

func (r *Renderer) renderLogPane(v view.View, vs view.ViewState) rty.Component {
	tabView := NewTabView(v, vs)
	var height int
	switch vs.TiltLogState {
	case view.TiltLogShort:
		height = defaultLogPaneHeight
	case view.TiltLogHalfScreen:
		height = rty.GROW
	case view.TiltLogFullScreen:
		height = 1
		// FullScreen is handled elsewhere, since it's no longer a pane
		// but we have to set height to something non-0 or rty will blow up
	}
	return rty.NewFixedSize(tabView.Build(), rty.GROW, height)
}

func renderPaneHeader(isMax bool) rty.Component {
	var verb string
	if isMax {
		verb = "contract"
	} else {
		verb = "expand"
	}
	s := fmt.Sprintf("X: %s", verb)
	l := rty.NewLine()
	l.Add(rty.NewFillerString(' '))
	l.Add(rty.TextString(fmt.Sprintf(" %s ", s)))
	return l
}

func (r *Renderer) renderStatusBar(v view.View) rty.Component {
	sb := rty.NewStringBuilder()
	sb.Text(" ") // Indent
	errorCount := 0
	for _, res := range v.Resources {
		if isInError(res) {
			errorCount++
		}
	}
	if errorCount == 0 && v.TiltfileErrorMessage() == "" {
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

		if v.TiltfileErrorMessage() != "" {
			_, _ = tiltfileError.WriteString(" • Tiltfile error")
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
	if vs.AlertMessage != "" {
		return "Tilt (l)og ┊ (esc) close alert "
	}
	return defaultKeys
}

func isInError(res view.Resource) bool {
	return statusDisplayOptions(res).color == cBad
}

func isCrashing(res view.Resource) bool {
	return (res.IsK8s() && res.K8sInfo().PodRestarts > 0) ||
		res.LastBuild().Reason.Has(model.BuildReasonFlagCrash) ||
		res.CurrentBuild.Reason.Has(model.BuildReasonFlagCrash) ||
		res.PendingBuildReason.Has(model.BuildReasonFlagCrash) ||
		res.IsDC() && res.DockerComposeTarget().Status() == string(dockercompose.StatusCrash)
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

	k8sCell := rty.ColoredString(" CONTAINER", cLightText)
	l.Add(k8sCell)
	l.Add(middotText())

	buildCell := rty.NewMinLengthLayout(BuildDurCellMinWidth+BuildStatusCellMinWidth, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(rty.ColoredString("UPDATE STATUS ", cLightText))
	l.Add(buildCell)
	l.Add(middotText())
	deployCell := rty.NewMinLengthLayout(DeployCellMinWidth+1, rty.DirHor).
		SetAlign(rty.AlignEnd).
		Add(rty.ColoredString("AS OF ", cLightText))
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
			resView := NewResourceView(v.LogReader, res, vs.Resources[i], res.TriggerMode, selectedResource == res.Name.String(), r.clock)
			l.Add(resView.Build())
		}
	}

	cl.Add(l)
	return cl
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

	r.rty = rty.NewRTY(screen, rty.SkipErrorHandler{})

	r.screen = screen

	return screenEvents, nil
}

func (r *Renderer) RTY() rty.RTY {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.rty
}

func (r *Renderer) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.screen != nil {
		r.screen.Fini()
	}

	r.screen = nil
}
