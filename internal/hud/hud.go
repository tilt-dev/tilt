package hud

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

type HeadsUpDisplay interface {
	store.Subscriber

	Run(ctx context.Context, st *store.Store) error
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

func (h *Hud) Run(ctx context.Context, st *store.Store) error {
	a, err := NewServer(ctx)
	if err != nil {
		return err
	}

	h.a = a

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
			err := h.r.SetUp(ready, st)
			if err != nil {
				return err
			}
		case <-a.streamClosedCh:
			h.r.Reset()
		case <-a.serverClosed:
			return nil
		}
	}
}

func (h *Hud) Close() {
	if h.a != nil {
		h.a.Close()
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
