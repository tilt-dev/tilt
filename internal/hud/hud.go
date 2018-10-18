package hud

import (
	"context"
	"sync"
	"time"

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

	Run(ctx context.Context, st *store.Store, refreshRate time.Duration) error
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

func NewDefaultHeadsUpDisplay() (HeadsUpDisplay, error) {
	return &Hud{
		r: NewRenderer(),
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

func (h *Hud) Run(ctx context.Context, st *store.Store, refreshRate time.Duration) error {
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
			screenEvents, err = h.r.SetUp(ready)
			if err != nil {
				return err
			}
		case <-a.streamClosedCh:
			h.r.Reset()
		case e := <-screenEvents:
			h.handleScreenEvent(ctx, st, e)
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

func (h *Hud) handleScreenEvent(ctx context.Context, st *store.Store, ev tcell.Event) {
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
				st.Dispatch(NewShowErrorAction(int(r - '0')))
			}
		case tcell.KeyUp:
			h.r.rty.ElementScroller("resources").UpElement()
			h.refresh(ctx)
		case tcell.KeyDown:
			h.r.rty.ElementScroller("resources").DownElement()
			h.refresh(ctx)
		case tcell.KeyEnter:
			activeItem := h.r.rty.ElementScroller("resources").GetSelectedIndex()
			st.Dispatch(NewShowErrorAction(activeItem + 1))
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

var _ store.Subscriber = &Hud{}
