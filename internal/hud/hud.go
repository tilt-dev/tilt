package hud

import (
	"context"
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
	a *ServerAdapter
	r *Renderer

	currentView view.View
	viewState   view.ViewState
	mu          sync.RWMutex
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
	return r.TextScroller("logmodal")
}

func (h *Hud) Run(ctx context.Context, dispatch func(action store.Action), refreshRate time.Duration) error {
	a, err := NewServer(ctx)
	if err != nil {
		return err
	}

	h.a = a

	if refreshRate == 0 {
		refreshRate = DefaultRefreshInterval
	}
	ticker := time.NewTicker(refreshRate)

	var screenEvents chan tcell.Event
	for {
		select {
		case <-ctx.Done():
			h.Close()
			err := ctx.Err()
			if err != context.Canceled {
				return err
			} else {
				return nil
			}
		case ready := <-a.readyCh:
			screenEvents, err = h.r.SetUp(ready, a.winchCh)
			if err != nil {
				return err
			}
			h.Refresh(ctx)
		case <-a.winchCh:
			h.Refresh(ctx)
		case <-a.streamClosedCh:
			h.r.Reset()
		case e := <-screenEvents:
			h.handleScreenEvent(ctx, dispatch, e)
		case <-a.serverClosed:
			return nil
		case <-ticker.C:
			h.Refresh(ctx)
		}
	}
}

func (h *Hud) Close() {
	if h.a != nil {
		h.a.Close()
	}
	h.r.Reset()
}

func (h *Hud) handleScreenEvent(ctx context.Context, dispatch func(action store.Action), ev tcell.Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	switch ev := ev.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyEscape:
			h.Close()
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
			}
		case tcell.KeyUp:
			h.selectedScroller(h.r.rty).Up()
			h.refresh(ctx)
		case tcell.KeyDown:
			h.selectedScroller(h.r.rty).Down()
			h.refresh(ctx)
		case tcell.KeyHome:
			h.selectedScroller(h.r.rty).Top()
		case tcell.KeyEnd:
			h.selectedScroller(h.r.rty).Bottom()
		case tcell.KeyEnter:
			if h.viewState.DisplayedLogNumber == 0 {
				selectedIdx, _ := h.selectedResource()
				h.viewState.DisplayedLogNumber = selectedIdx + 1
				logModal(h.r.rty).Bottom()
			} else {
				h.viewState.DisplayedLogNumber = 0
			}
			h.refresh(ctx)
		}
	}
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
const logScrollerName = "logmodal"

func (h *Hud) selectedScroller(rty rty.RTY) Scroller {
	if h.viewState.DisplayedLogNumber == 0 {
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
