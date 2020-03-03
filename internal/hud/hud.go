package hud

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"

	"github.com/windmilleng/tilt/internal/output"
	"github.com/windmilleng/tilt/pkg/logger"

	"github.com/gdamore/tcell"
	"github.com/pkg/browser"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

// The main loop ensures the HUD updates at least this often
const DefaultRefreshInterval = 100 * time.Millisecond

// number of arrows a pgup/dn is equivalent to
// (we don't currently worry about trying to know how big a page is, and instead just support pgup/dn as "faster arrows"
const pgUpDownCount = 20

type HudEnabled bool

type HeadsUpDisplay interface {
	store.Subscriber

	Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error
}

type Hud struct {
	r      *Renderer
	webURL model.WebURL

	currentView      view.View
	currentViewState view.ViewState
	mu               sync.RWMutex
	isRunning        bool
	a                *analytics.TiltAnalytics
}

var _ HeadsUpDisplay = (*Hud)(nil)

func ProvideHud(hudEnabled HudEnabled, renderer *Renderer, webURL model.WebURL, analytics *analytics.TiltAnalytics, printer *IncrementalPrinter) (HeadsUpDisplay, error) {
	if !hudEnabled {
		return NewDisabledHud(printer), nil
	}
	return NewDefaultHeadsUpDisplay(renderer, webURL, analytics)
}

func NewDefaultHeadsUpDisplay(renderer *Renderer, webURL model.WebURL, analytics *analytics.TiltAnalytics) (HeadsUpDisplay, error) {
	return &Hud{
		r:      renderer,
		webURL: webURL,
		a:      analytics,
	}, nil
}

func (h *Hud) SetNarrationMessage(ctx context.Context, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	currentViewState := h.currentViewState
	currentViewState.ShowNarration = true
	currentViewState.NarrationMessage = msg
	h.setViewState(ctx, currentViewState)
}

func (h *Hud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	// Redirect stdout and stderr into our logger
	err := output.CaptureAllOutput(logger.Get(ctx).Writer(logger.InfoLvl))
	if err != nil {
		logger.Get(ctx).Infof("Error capturing stdout and stderr: %v", err)
	}

	h.mu.Lock()
	h.isRunning = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		h.isRunning = false
		h.mu.Unlock()
	}()

	screenEvents, err := h.r.SetUp()

	if err != nil {
		return errors.Wrap(err, "setting up screen")
	}

	defer h.Close()

	if refreshRate == 0 {
		refreshRate = DefaultRefreshInterval
	}
	ticker := time.NewTicker(refreshRate)

	for {
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if err != context.Canceled {
				return err
			}
			return nil
		case e := <-screenEvents:
			done := h.handleScreenEvent(ctx, dispatch, e)
			if done {
				return nil
			}
		case <-ticker.C:
			h.Refresh(ctx)
		}
	}
}

func (h *Hud) Close() {
	h.r.Reset()
}

func (h *Hud) recordInteraction(name string) {
	h.a.Incr(fmt.Sprintf("ui.interactions.%s", name), map[string]string{})
}

func (h *Hud) handleScreenEvent(ctx context.Context, dispatch func(action store.Action), ev tcell.Event) (done bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	escape := func() {
		am := h.activeModal()
		if am != nil {
			am.Close(&h.currentViewState)
		}
	}

	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEscape:
			escape()
		case tcell.KeyRune:
			switch r := ev.Rune(); {
			case r == 'b': // [B]rowser
				// If we have an endpoint(s), open the first one
				// TODO(nick): We might need some hints on what load balancer to
				// open if we have multiple, or what path to default to on the opened manifest.
				_, selected := h.selectedResource()
				if len(selected.Endpoints) > 0 {
					h.recordInteraction("open_preview")
					err := browser.OpenURL(selected.Endpoints[0])
					if err != nil {
						h.currentViewState.AlertMessage = fmt.Sprintf("error opening url '%s' for resource '%s': %v",
							selected.Endpoints[0], selected.Name, err)
					}
				} else {
					h.currentViewState.AlertMessage = fmt.Sprintf("no urls for resource '%s' ¯\\_(ツ)_/¯", selected.Name)
				}
			case r == 'l': // Tilt [L]og
				if h.webURL.Empty() {
					break
				}
				url := h.webURL
				url.Path = "/"
				_ = browser.OpenURL(url.String())
			case r == 'k':
				h.activeScroller().Up()
				h.refreshSelectedIndex()
			case r == 'j':
				h.activeScroller().Down()
				h.refreshSelectedIndex()
			case r == 'q': // [Q]uit
				escape()
			case r == 'R': // hidden key for recovering from printf junk during demos
				h.r.screen.Sync()
			case r == 'x':
				h.recordInteraction("cycle_view_log_state")
				h.currentViewState.CycleViewLogState()
			case r == '1':
				h.recordInteraction("tab_all_log")
				h.currentViewState.TabState = view.TabAllLog
			case r == '2':
				h.recordInteraction("tab_build_log")
				h.currentViewState.TabState = view.TabBuildLog
			case r == '3':
				h.recordInteraction("tab_pod_log")
				h.currentViewState.TabState = view.TabRuntimeLog
			}
		case tcell.KeyUp:
			h.activeScroller().Up()
			h.refreshSelectedIndex()
		case tcell.KeyDown:
			h.activeScroller().Down()
			h.refreshSelectedIndex()
		case tcell.KeyPgUp:
			for i := 0; i < pgUpDownCount; i++ {
				h.activeScroller().Up()
			}
			h.refreshSelectedIndex()
		case tcell.KeyPgDn:
			for i := 0; i < pgUpDownCount; i++ {
				h.activeScroller().Down()
			}
			h.refreshSelectedIndex()
		case tcell.KeyEnter:
			if len(h.currentView.Resources) == 0 {
				break
			}
			_, r := h.selectedResource()

			if h.webURL.Empty() {
				break
			}
			url := h.webURL
			url.Path = fmt.Sprintf("/r/%s/", r.Name)
			h.a.Incr("ui.interactions.open_log", map[string]string{"is_tiltfile": strconv.FormatBool(r.Name == store.TiltfileManifestName)})
			_ = browser.OpenURL(url.String())
		case tcell.KeyRight:
			i, _ := h.selectedResource()
			h.currentViewState.Resources[i].CollapseState = view.CollapseNo
		case tcell.KeyLeft:
			i, _ := h.selectedResource()
			h.currentViewState.Resources[i].CollapseState = view.CollapseYes
		case tcell.KeyHome:
			h.activeScroller().Top()
		case tcell.KeyEnd:
			h.activeScroller().Bottom()
		case tcell.KeyCtrlC:
			h.Close()
			dispatch(NewExitAction(nil))
			return true
		case tcell.KeyCtrlD:
			dispatch(DumpEngineStateAction{})
		case tcell.KeyCtrlP:
			if h.currentView.IsProfiling {
				dispatch(StopProfilingAction{})
			} else {
				dispatch(StartProfilingAction{})
			}
		case tcell.KeyCtrlO:
			go writeHeapProfile(ctx)
		}

	case *tcell.EventResize:
		// since we already refresh after the switch, don't need to do anything here
		// just marking this as where sigwinch gets handled
	}

	h.refresh(ctx)
	return false
}

func (h *Hud) OnChange(ctx context.Context, st store.RStore) {
	h.mu.Lock()
	defer h.mu.Unlock()

	toPrint := ""

	state := st.RLockState()
	view := store.StateToView(state, st.StateMutex())

	// if the hud isn't running, make sure new logs are visible on stdout
	if !h.isRunning {
		toPrint = state.LogStore.ContinuingString(h.currentViewState.ProcessedLogs)
	}
	h.currentViewState.ProcessedLogs = state.LogStore.Checkpoint()

	st.RUnlockState()

	fmt.Print(toPrint)

	// if we're going from 1 resource (i.e., the Tiltfile) to more than 1, reset
	// the resource selection, so that we're not scrolled to the bottom with the Tiltfile selected
	if len(h.currentView.Resources) == 1 && len(view.Resources) > 1 {
		h.resetResourceSelection()
	}
	h.currentView = view
	h.refreshSelectedIndex()
}

func (h *Hud) Refresh(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) setViewState(ctx context.Context, currentViewState view.ViewState) {
	h.currentViewState = currentViewState
	h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) refresh(ctx context.Context) {
	// TODO: We don't handle the order of resources changing
	for len(h.currentViewState.Resources) < len(h.currentView.Resources) {
		h.currentViewState.Resources = append(h.currentViewState.Resources, view.ResourceViewState{})
	}

	vs := h.currentViewState
	vs.Resources = append(vs.Resources, h.currentViewState.Resources...)

	h.r.Render(h.currentView, h.currentViewState)
}

func (h *Hud) resetResourceSelection() {
	rty := h.r.RTY()
	if rty == nil {
		return
	}
	// wipe out any scroll/selection state for resources
	// it will get re-set in the next call to render
	rty.RegisterElementScroll("resources", []string{})
}

func (h *Hud) refreshSelectedIndex() {
	rty := h.r.RTY()
	if rty == nil {
		return
	}
	scroller := rty.ElementScroller("resources")
	if scroller == nil {
		return
	}
	i := scroller.GetSelectedIndex()
	h.currentViewState.SelectedIndex = i
}

func (h *Hud) selectedResource() (i int, resource view.Resource) {
	return selectedResource(h.currentView, h.currentViewState)
}

func selectedResource(view view.View, state view.ViewState) (i int, resource view.Resource) {
	i = state.SelectedIndex
	if i >= 0 && i < len(view.Resources) {
		resource = view.Resources[i]
	}
	return i, resource

}

var _ store.Subscriber = &Hud{}

func writeHeapProfile(ctx context.Context) {
	f, err := os.Create("tilt.heap_profile")
	if err != nil {
		logger.Get(ctx).Infof("error creating file for heap profile: %v", err)
		return
	}
	runtime.GC()
	logger.Get(ctx).Infof("writing heap profile to %s", f.Name())
	err = pprof.WriteHeapProfile(f)
	if err != nil {
		logger.Get(ctx).Infof("error writing heap profile: %v", err)
		return
	}
	err = f.Close()
	if err != nil {
		logger.Get(ctx).Infof("error closing file for heap profile: %v", err)
		return
	}
	logger.Get(ctx).Infof("wrote heap profile to %s", f.Name())
}
