package hud

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/windmilleng/tilt/internal/rty"

	"github.com/pkg/browser"
	"github.com/windmilleng/tcell"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

// The main loop ensures the HUD updates at least this often
const DefaultRefreshInterval = 1 * time.Second

type HeadsUpDisplay interface {
	store.Subscriber

	Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error
	Update(v view.View) error
	Close()
	SetNarrationMessage(ctx context.Context, msg string)
}

type Hud struct {
	r *Renderer

	currentView view.View
	viewState   view.ViewState
	mu          sync.RWMutex
	isRunning   bool
}

var _ HeadsUpDisplay = (*Hud)(nil)

func NewDefaultHeadsUpDisplay(renderer *Renderer) (HeadsUpDisplay, error) {
	return &Hud{
		r: renderer,
	}, nil
}

func (h *Hud) SetNarrationMessage(ctx context.Context, msg string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	viewState := h.viewState
	viewState.ShowNarration = true
	viewState.NarrationMessage = msg
	h.setViewState(ctx, viewState)
}

func logModal(r rty.RTY) rty.TextScroller {
	return r.TextScroller(logScrollerName)
}

func (h *Hud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
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
		return errors.Wrap(err, "error initializing renderer")
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
			} else {
				return nil
			}
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

func (h *Hud) handleScreenEvent(ctx context.Context, dispatch func(action store.Action), ev tcell.Event) (done bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEscape:
			h.viewState.LogModal = view.LogModal{}
		case tcell.KeyRune:
			switch r := ev.Rune(); {
			case r >= '1' && r <= '9':
				dispatch(NewShowErrorAction(int(r - '0')))
			case r == 'b': // "[B]rowser
				// If we have an endpoint(s), open the first one
				// TODO(nick): We might need some hints on what load balancer to
				// open if we have multiple, or what path to default to on the opened manifest.
				_, selected := h.selectedResource()
				if len(selected.Endpoints) > 0 {
					err := browser.OpenURL(selected.Endpoints[0])
					if err != nil {
						logger.Get(ctx).Infof("error opening url '%s' for resource '%s': %v",
							selected.Endpoints[0], selected.Name, err)
					}
				} else {
					logger.Get(ctx).Infof("no urls for resource '%s' ¯\\_(ツ)_/¯", selected.Name)
				}
			case r == 'q': // [Q]uit
				h.Close()
				dispatch(ExitAction{})
				return true
			case r == 'l': // [L]og
				if !h.viewState.LogModal.IsActive() {
					h.viewState.LogModal = view.LogModal{TiltLog: true}
				}
				logModal(h.r.rty).Bottom()
			}
		case tcell.KeyCtrlC:
			h.Close()
			dispatch(ExitAction{})
			return true
		case tcell.KeyUp:
			h.selectedScroller(h.r.rty).Up()
		case tcell.KeyDown:
			h.selectedScroller(h.r.rty).Down()
		case tcell.KeyHome:
			h.selectedScroller(h.r.rty).Top()
		case tcell.KeyEnd:
			h.selectedScroller(h.r.rty).Bottom()
		case tcell.KeyEnter:
			if !h.viewState.LogModal.IsActive() {
				selectedIdx, _ := h.selectedResource()
				h.viewState.LogModal = view.LogModal{ResourceLogNumber: selectedIdx + 1}
				logModal(h.r.rty).Bottom()
			}
		}
	case *tcell.EventResize:
		// since we already refresh after the switch, don't need to do anything here
		// just marking this as where sigwinch gets handled
	}

	h.refresh(ctx)

	return false
}

func (h *Hud) OnChange(ctx context.Context, st *store.Store) {
	state := st.RLockState()
	view := store.StateToView(state)
	st.RUnlockState()

	h.mu.Lock()
	defer h.mu.Unlock()
	h.setView(ctx, view)
}

func (h *Hud) Refresh(ctx context.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) setView(ctx context.Context, view view.View) {
	h.currentView = view

	// if the hud isn't running, make sure new logs are visible on stdout
	if !h.isRunning && h.viewState.ProcessedLogByteCount < len(view.Log) {
		fmt.Print(view.Log[h.viewState.ProcessedLogByteCount:])
	}

	h.viewState.ProcessedLogByteCount = len(view.Log)

	h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) setViewState(ctx context.Context, viewState view.ViewState) {
	h.viewState = viewState
	h.refresh(ctx)
}

// Must hold the lock
func (h *Hud) refresh(ctx context.Context) {
	h.currentView.ViewState = h.viewState

	err := h.Update(h.currentView)
	if err != nil {
		logger.Get(ctx).Infof("Error updating HUD: %v", err)
	}
}

func (h *Hud) Update(v view.View) error {
	err := h.r.Render(v)
	return errors.Wrap(err, "error rendering hud")
}

func (h *Hud) selectedResource() (i int, resource view.Resource) {
	i = h.r.rty.ElementScroller("resources").GetSelectedIndex()
	if i >= 0 && i < len(h.currentView.Resources) {
		resource = h.currentView.Resources[i]
	}
	return i, resource
}

var _ store.Subscriber = &Hud{}

const resourcesScollerName = "resources"
const logScrollerName = "log modal"

func (h *Hud) selectedScroller(rty rty.RTY) Scroller {
	if !h.viewState.LogModal.IsActive() {
		return h.r.rty.ElementScroller(resourcesScollerName)
	} else {
		return h.r.rty.TextScroller(logScrollerName)
	}
}

type Scroller interface {
	Up()
	Down()
	Top()
	Bottom()
}
