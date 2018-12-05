package hud

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/windmilleng/tcell"
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
	// A build is in progress, triggered by an explicit edit.
	if res.CurrentBuildStartTime.After(res.LastBuildFinishTime) &&
		!res.CurrentBuildReason.IsCrashOnly() {
		return res.CurrentBuildLog
	}

	// The last build was an error.
	if res.LastBuildError != "" {
		return res.LastBuildLog
	}

	// Two cases:
	// 1) The last build finished before this pod started
	// 2) This log is from an in-place container update.
	// in either case, prepend them to pod logs.
	if (res.LastBuildStartTime.Equal(res.PodUpdateStartTime) ||
		res.LastBuildStartTime.Before(res.PodCreationTime)) &&
		len(strings.TrimSpace(res.LastBuildLog)) > 0 {
		return res.LastBuildLog + "\n" + res.PodLog
	}

	// The last build finished, but the pod hasn't started yet.
	if res.LastBuildStartTime.After(res.PodCreationTime) {
		return res.LastBuildLog
	}

	return res.PodLog
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

	cl := rty.NewConcatLayout(rty.DirVert)
	cl.Add(r.renderTiltfileError(v))

	childNames := make([]string, len(rs))
	for i, r := range rs {
		childNames[i] = r.Name
	}
	// the items added to `l` below must be kept in sync with `childNames` above
	l, selectedResource := r.rty.RegisterElementScroll(resourcesScollerName, childNames)

	if len(rs) > 0 {
		for i, res := range rs {
			l.Add(r.renderResource(res, vs.Resources[i], selectedResource == res.Name))
		}
	}

	cl.Add(l)
	return cl
}

var spinnerChars = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (r *Renderer) spinner() string {
	return spinnerChars[r.clock().Second()%len(spinnerChars)]
}

func (r *Renderer) renderResource(res view.Resource, rv view.ResourceViewState, selected bool) rty.Component {
	return NewResourceView(res, rv, selected).Build()
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
