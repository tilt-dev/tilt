package hud

import (
	"context"
	"testing"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/hud/view"
)

var _ HeadsUpDisplay = (*FakeHud)(nil)

type FakeHud struct {
	LastView view.View
	updates  chan view.View
	Canceled bool
	Closed   bool
	closeCh  chan interface{}
}

func NewFakeHud() *FakeHud {
	return &FakeHud{
		updates: make(chan view.View, 10),
		closeCh: make(chan interface{}),
	}
}

func (h *FakeHud) Run(ctx context.Context, dispatch func(action store.Action), refreshInterval time.Duration) error {
	select {
	case <-ctx.Done():
	case <-h.closeCh:
	}
	h.Canceled = true
	return ctx.Err()
}

func (h *FakeHud) SetNarrationMessage(ctx context.Context, msg string) {}
func (h *FakeHud) Refresh(ctx context.Context)                         {}

func (h *FakeHud) OnChange(ctx context.Context, st *store.Store) {
	state := st.RLockState()
	view := store.StateToView(state)
	st.RUnlockState()

	err := h.Update(view)
	if err != nil {
		logger.Get(ctx).Infof("Error updating HUD: %v", err)
	}
}

func (h *FakeHud) Close() {
	h.Closed = true
	close(h.closeCh)
}

func (h *FakeHud) Update(v view.View) error {
	h.LastView = v
	h.updates <- v
	return nil
}

func (h *FakeHud) WaitUntil(t testing.TB, ctx context.Context, msg string, isDone func(view.View) bool) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			t.Fatalf("Timed out waiting for: %s", msg)
		case view := <-h.updates:
			done := isDone(view)
			if done {
				return
			}
		}
	}
}
